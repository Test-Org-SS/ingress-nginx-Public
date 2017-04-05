package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	compute "google.golang.org/api/compute/v1"
	"k8s.io/kubernetes/pkg/cloudprovider"
	"k8s.io/kubernetes/pkg/cloudprovider/providers/gce"
)

var (
	projectID           string
	regionName          string
	targetBalancingMode string

	instanceGroupName string

	s         *compute.Service
	g         *gce.GCECloud
	zones     []*compute.Zone
	igs       map[string]*compute.InstanceGroup
	instances []*compute.Instance
)

const (
	instanceGroupTemp = "k8s-ig--migrate"
	balancingModeRATE = "RATE"
	balancingModeUTIL = "UTILIZATION"

	version = 0.1
)

func main() {
	fmt.Println("Backend-Service BalancingMode Updater", "version:", version)
	//flag.Usage
	flag.Parse()

	args := flag.Args()
	if len(args) != 3 {
		log.Fatalf("Expected three arguments: project_id region balancing_mode")
	}
	projectID, regionName, targetBalancingMode = args[0], args[1], args[2]

	switch targetBalancingMode {
	case balancingModeRATE, balancingModeUTIL:
	default:
		panic(fmt.Errorf("expected either %s or %s, actual: %v", balancingModeRATE, balancingModeUTIL, targetBalancingMode))
	}

	switch regionName {
	case "asia-east1", "asia-northeast1", "europe-west1", "us-central1", "us-east1", "us-west1":
	default:
		panic(fmt.Errorf("expected a valid GCP region, actual: %v", regionName))
	}

	igs = make(map[string]*compute.InstanceGroup)

	tokenSource, err := google.DefaultTokenSource(
		oauth2.NoContext,
		compute.CloudPlatformScope,
		compute.ComputeScope)
	if err != nil {
		panic(err)
	}

	client := oauth2.NewClient(oauth2.NoContext, tokenSource)
	s, err = compute.New(client)
	if err != nil {
		panic(err)
	}

	cloudInterface, err := cloudprovider.GetCloudProvider("gce", nil)
	if err != nil {
		panic(err)
	}
	g = cloudInterface.(*gce.GCECloud)

	// Get Zones
	zoneFilter := fmt.Sprintf("(region eq %s)", createRegionLink(regionName))
	zoneList, err := s.Zones.List(projectID).Filter(zoneFilter).Do()
	if err != nil {
		panic(err)
	}
	zones = zoneList.Items

	// Get instance groups
	for _, z := range zones {
		igl, err := s.InstanceGroups.List(projectID, z.Name).Do()
		if err != nil {
			panic(err)
		}
		for _, ig := range igl.Items {
			if !strings.HasPrefix(ig.Name, "k8s-ig--") {
				continue
			}

			if instanceGroupName == "" {
				instanceGroupName = ig.Name
			}

			// Note instances
			r := &compute.InstanceGroupsListInstancesRequest{InstanceState: "ALL"}
			instList, err := s.InstanceGroups.ListInstances(projectID, getResourceName(ig.Zone, "zones"), ig.Name, r).Do()
			if err != nil {
				panic(err)
			}

			for _, i := range instList.Items {
				inst, err := s.Instances.Get(projectID, getResourceName(ig.Zone, "zones"), getResourceName(i.Instance, "instances")).Do()
				if err != nil {
					panic(err)
				}

				instances = append(instances, inst)
			}

			// Note instance group in zone
			igs[z.Name] = ig
		}
	}

	if instanceGroupName == "" {
		panic(errors.New("Could not determine k8s load balancer instance group"))
	}

	bs := getBackendServices()
	fmt.Println("Backend Services:", len(bs))
	fmt.Println("Instance Groups:", len(igs))

	// Create temoprary instance groups
	for zone, ig := range igs {
		_, err = s.InstanceGroups.Get(projectID, zone, instanceGroupTemp).Do()
		if err != nil {
			newIg := &compute.InstanceGroup{
				Name:       instanceGroupTemp,
				Zone:       zone,
				NamedPorts: ig.NamedPorts,
			}
			fmt.Println("Creating", instanceGroupTemp, "zone:", zone)
			_, err = s.InstanceGroups.Insert(projectID, zone, newIg).Do()
			if err != nil {
				panic(err)
			}
		}
	}

	// Straddle both groups
	fmt.Println("Straddle both groups in backend services")
	setBackendsTo(true, balancingModeInverse(targetBalancingMode), true, balancingModeInverse(targetBalancingMode))

	fmt.Println("Migrate instances to temporary group")
	migrateInstances(instanceGroupName, instanceGroupTemp)

	time.Sleep(20 * time.Second)

	// Remove original backends
	fmt.Println("Remove original backends")
	setBackendsTo(false, "", true, balancingModeInverse(targetBalancingMode))

	sleep(1 * time.Minute)

	// Straddle both groups (new balancing mode)
	fmt.Println("Create backends pointing to original instance groups")
	setBackendsTo(true, targetBalancingMode, true, balancingModeInverse(targetBalancingMode))

	sleep(20 * time.Second)

	fmt.Println("Migrate instances back to original groups")
	migrateInstances(instanceGroupTemp, instanceGroupName)

	sleep(20 * time.Second)

	fmt.Println("Remove temporary backends")
	setBackendsTo(true, targetBalancingMode, false, "")

	sleep(20 * time.Second)

	fmt.Println("Delete temporary instance groups")
	for z := range igs {
		_, err = s.InstanceGroups.Delete(projectID, z, instanceGroupTemp).Do()
		if err != nil {
			fmt.Println("Couldn't delete temporary instance group", instanceGroupTemp)
		}
	}
}

func sleep(d time.Duration) {
	fmt.Println("Sleeping for", d.String())
	time.Sleep(d)
}

func setBackendsTo(orig bool, origMode string, temp bool, tempMode string) {
	bs := getBackendServices()
	for _, bsi := range bs {
		var union []*compute.Backend
		for zone := range igs {
			if orig {
				b := &compute.Backend{
					Group:              createInstanceGroupLink(zone, instanceGroupName),
					BalancingMode:      origMode,
					CapacityScaler:     0.8,
					MaxRatePerInstance: 1.0,
				}
				union = append(union, b)
			}
			if temp {
				b := &compute.Backend{
					Group:              createInstanceGroupLink(zone, instanceGroupTemp),
					BalancingMode:      tempMode,
					CapacityScaler:     0.8,
					MaxRatePerInstance: 1.0,
				}
				union = append(union, b)
			}
		}
		bsi.Backends = union
		_, err := s.BackendServices.Update(projectID, bsi.Name, bsi).Do()
		if err != nil {
			panic(err)
		}
	}
}

func balancingModeInverse(m string) string {
	switch m {
	case balancingModeRATE:
		return balancingModeUTIL
	case balancingModeUTIL:
		return balancingModeRATE
	default:
		return ""
	}
}

func getBackendServices() (bs []*compute.BackendService) {
	bsl, err := s.BackendServices.List(projectID).Do()
	if err != nil {
		panic(err)
	}

	for _, bsli := range bsl.Items {
		// Ignore regional backend-services and only grab Kubernetes resources
		if bsli.Region == "" && strings.HasPrefix(bsli.Name, "k8s-be-") {
			bs = append(bs, bsli)
		}
	}
	return bs
}

func migrateInstances(fromIG, toIG string) error {
	wg := sync.WaitGroup{}
	for _, i := range instances {
		wg.Add(1)
		go func(i *compute.Instance) {
			z := getResourceName(i.Zone, "zones")
			fmt.Printf(" - %s (%s)\n", i.Name, z)
			rr := &compute.InstanceGroupsRemoveInstancesRequest{Instances: []*compute.InstanceReference{{Instance: i.SelfLink}}}
			_, err := s.InstanceGroups.RemoveInstances(projectID, z, fromIG, rr).Do()
			if err != nil {
				fmt.Println("Skipping error when removing instance from group", err)
			}
			time.Sleep(10 * time.Second)

			ra := &compute.InstanceGroupsAddInstancesRequest{Instances: []*compute.InstanceReference{{Instance: i.SelfLink}}}
			_, err = s.InstanceGroups.AddInstances(projectID, z, toIG, ra).Do()
			if err != nil {
				if !strings.Contains(err.Error(), "memberAlreadyExists") { // GLBC already added the instance back to the IG
					fmt.Println("failed to add instance to new IG", i.Name, err)
				}
			}
			wg.Done()
		}(i)
		time.Sleep(10 * time.Second)
	}
	wg.Wait()
	return nil
}

func createInstanceGroupLink(zone, igName string) string {
	return fmt.Sprintf("https://www.googleapis.com/compute/v1/projects/%s/zones/%s/instanceGroups/%s", projectID, zone, igName)
}

func createRegionLink(region string) string {
	return fmt.Sprintf("https://www.googleapis.com/compute/v1/projects/nicksardo-playground/regions/%v", region)
}

func getResourceName(link string, resourceType string) string {
	s := strings.Split(link, "/")

	for i := 0; i < len(s); i++ {
		if s[i] == resourceType {
			if i+1 <= len(s) {
				return s[i+1]
			}
		}
	}
	return ""
}
