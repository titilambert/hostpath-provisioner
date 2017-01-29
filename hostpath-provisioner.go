package main

import (
	"errors"
	"flag"
	"os"
	"path"
	"time"
    "fmt"

	"github.com/golang/glog"
	"github.com/kubernetes-incubator/nfs-provisioner/controller"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/pkg/types"
	"k8s.io/client-go/pkg/util/uuid"
	"k8s.io/client-go/pkg/util/wait"
	"k8s.io/client-go/rest"
    "k8s.io/client-go/tools/clientcmd"
)

var (
    provisionerName         = flag.String("provisioner", "example.com/hostpath", "Name of the provisioner. The provisioner will only provision volumes for claims that request a StorageClass with a provisioner field set equal to this name.")
    pvRootDir               = flag.String("pv-root-dir", "/tmp/hostpath-provisioner", "Volumes root folder")
    master                  = flag.String("master", "", "Master URL to build a client config from. Either this or kubeconfig needs to be set if the provisioner is being run out of cluster.")
    kubeconfig              = flag.String("kubeconfig", "", "Absolute path to the kubeconfig file. Either this or master needs to be set if the provisioner is being run out of cluster.")
    defaultReclaimPolicyStr = flag.String("default-reclaim-policy", "Delete", "Default Reclaim Policy. Should be 'Retain' or 'Delete'")
)


const (
	resyncPeriod              = 15 * time.Second
	exponentialBackOffOnError = false
	failedRetryThreshold      = 5
)

type hostPathProvisioner struct {
	// The directory to create PV-backing directories in
	pvDir string
    //
    defaultReclaimPolicy v1.PersistentVolumeReclaimPolicy
	// Identity of this hostPathProvisioner, generated. Used to identify "this"
	// provisioner's PVs.
	identity types.UID
}

func NewHostPathProvisioner(pvDir string, defaultReclaimPolicy v1.PersistentVolumeReclaimPolicy) controller.Provisioner {
	return &hostPathProvisioner{
		pvDir:    pvDir,
        defaultReclaimPolicy: defaultReclaimPolicy,
		identity: uuid.NewUUID(),
	}
}

var _ controller.Provisioner = &hostPathProvisioner{}

// Provision creates a storage asset and returns a PV object representing it.
func (p *hostPathProvisioner) Provision(options controller.VolumeOptions) (*v1.PersistentVolume, error) {
	path := path.Join(p.pvDir, options.PVC.Namespace, options.PVC.Name)

    reclaimPolicyWanted, ok := options.PVC.Annotations["persistentVolumeReclaimPolicy"]
    var reclaimPolicy v1.PersistentVolumeReclaimPolicy
    if ok && reclaimPolicyWanted == "Retain" {
        reclaimPolicy = v1.PersistentVolumeReclaimRetain
    } else if ok && reclaimPolicyWanted == "Delete" {
        reclaimPolicy = v1.PersistentVolumeReclaimDelete

    } else {
        reclaimPolicy = p.defaultReclaimPolicy
    }

	if err := os.MkdirAll(path, 0777); err != nil {
		return nil, err
	}

	pv := &v1.PersistentVolume{
		ObjectMeta: v1.ObjectMeta{
			Name: options.PVName,
			Annotations: map[string]string{
				"hostPathProvisionerIdentity": string(p.identity),
			},
		},
		Spec: v1.PersistentVolumeSpec{
			PersistentVolumeReclaimPolicy: reclaimPolicy,
			AccessModes:                   options.PVC.Spec.AccessModes,
			Capacity: v1.ResourceList{
				v1.ResourceName(v1.ResourceStorage): options.PVC.Spec.Resources.Requests[v1.ResourceName(v1.ResourceStorage)],
			},
			PersistentVolumeSource: v1.PersistentVolumeSource{
				HostPath: &v1.HostPathVolumeSource{
					Path: path,
				},
			},
		},
	}

	return pv, nil
}

// Delete removes the storage asset that was created by Provision represented
// by the given PV.
func (p *hostPathProvisioner) Delete(volume *v1.PersistentVolume) error {
	ann, ok := volume.Annotations["hostPathProvisionerIdentity"]
	if !ok {
		return errors.New("identity annotation not found on PV")
	}
	if ann != string(p.identity) {
		return &controller.IgnoredError{"identity annotation on PV does not match ours"}
	}

	path := path.Join(p.pvDir, volume.Name)
	if err := os.RemoveAll(path); err != nil {
		return err
	}

	return nil
}

func main() {
	flag.Set("logtostderr", "true")
	flag.Parse()

	// Create an InClusterConfig and use it to create a client for the controller
	// to use to communicate with Kubernetes
    outOfCluster := *master != "" || *kubeconfig != ""
    var config *rest.Config
    var err error
    if outOfCluster {
        config, err = clientcmd.BuildConfigFromFlags(*master, *kubeconfig)
    } else {
        config, err = rest.InClusterConfig()
    }
    if err != nil {
        glog.Fatalf("Failed to create config: %v", err)
    }

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		glog.Fatalf("Failed to create client: %v", err)
	}
    var defaultReclaimPolicy v1.PersistentVolumeReclaimPolicy
    if *defaultReclaimPolicyStr == "Retain" {
        defaultReclaimPolicy = v1.PersistentVolumeReclaimRetain
    } else if *defaultReclaimPolicyStr == "Delete" {
        defaultReclaimPolicy = v1.PersistentVolumeReclaimDelete
    } else  {
		glog.Fatalf("default-reclaim-policy should be 'Delete' or 'Retain'")
    }

	// The controller needs to know what the server version is because out-of-tree
	// provisioners aren't officially supported until 1.5
	serverVersion, err := clientset.Discovery().ServerVersion()
	if err != nil {
		glog.Fatalf("Error getting server version: %v", err)
	}

	// Create the provisioner: it implements the Provisioner interface expected by
	// the controller
	hostPathProvisioner := NewHostPathProvisioner(*pvRootDir, defaultReclaimPolicy)

	// Start the provision controller which will dynamically provision hostPath
	// PVs
	pc := controller.NewProvisionController(clientset, resyncPeriod, *provisionerName, hostPathProvisioner, serverVersion.GitVersion, exponentialBackOffOnError, failedRetryThreshold)
	//pc := controller.NewProvisionController(clientset, resyncPeriod, *provisionerName, hostPathProvisioner, serverVersion.GitVersion, false, failedRetryThreshold)
	pc.Run(wait.NeverStop)
}
