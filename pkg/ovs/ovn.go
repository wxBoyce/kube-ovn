package ovs

import (
	"context"
	"errors"
	"time"

	"github.com/ovn-org/libovsdb/client"
	"github.com/ovn-org/libovsdb/ovsdb"
	"k8s.io/klog/v2"

	ovsclient "github.com/kubeovn/kube-ovn/pkg/ovsdb/client"
)

var (
	ErrNoAddr   = errors.New("no address")
	ErrNotFound = errors.New("not found")
)

// LegacyClient is the legacy ovn client
type LegacyClient struct {
	OvnNbAddress                  string
	OvnTimeout                    int
	OvnSbAddress                  string
	OVNIcNBAddress                string
	ClusterRouter                 string
	ClusterTcpLoadBalancer        string
	ClusterUdpLoadBalancer        string
	ClusterTcpSessionLoadBalancer string
	ClusterUdpSessionLoadBalancer string
	NodeSwitch                    string
	NodeSwitchCIDR                string
	ExternalGatewayType           string
	Version                       string
}

type OvnClient struct {
	ovnNbClient
}

type ovnNbClient struct {
	client.Client
	Timeout int
}

const (
	OvnNbCtl    = "ovn-nbctl"
	OvnSbCtl    = "ovn-sbctl"
	OVNIcNbCtl  = "ovn-ic-nbctl"
	OvsVsCtl    = "ovs-vsctl"
	MayExist    = "--may-exist"
	IfExists    = "--if-exists"
	Policy      = "--policy"
	PolicyDstIP = "dst-ip"
	PolicySrcIP = "src-ip"
)

// NewLegacyClient init a legacy ovn client
func NewLegacyClient(ovnNbAddr string, ovnNbTimeout int, ovnSbAddr, clusterRouter, clusterTcpLoadBalancer, clusterUdpLoadBalancer, clusterTcpSessionLoadBalancer, clusterUdpSessionLoadBalancer, nodeSwitch, nodeSwitchCIDR string) *LegacyClient {
	return &LegacyClient{
		OvnNbAddress:                  ovnNbAddr,
		OvnSbAddress:                  ovnSbAddr,
		OvnTimeout:                    ovnNbTimeout,
		ClusterRouter:                 clusterRouter,
		ClusterTcpLoadBalancer:        clusterTcpLoadBalancer,
		ClusterUdpLoadBalancer:        clusterUdpLoadBalancer,
		ClusterTcpSessionLoadBalancer: clusterTcpSessionLoadBalancer,
		ClusterUdpSessionLoadBalancer: clusterUdpSessionLoadBalancer,
		NodeSwitch:                    nodeSwitch,
		NodeSwitchCIDR:                nodeSwitchCIDR,
	}
}

// TODO: support sb/ic-nb client
func NewOvnClient(ovnNbAddr string, ovnNbTimeout int) (*OvnClient, error) {
	nbClient, err := ovsclient.NewNbClient(ovnNbAddr, ovnNbTimeout)
	if err != nil {
		klog.Errorf("failed to create OVN NB client: %v", err)
		return nil, err
	}

	return &OvnClient{ovnNbClient: ovnNbClient{Client: nbClient, Timeout: ovnNbTimeout}}, nil
}

func Transact(c client.Client, method string, operations []ovsdb.Operation, timeout int) error {
	ctx, cancel := context.WithTimeout(context.TODO(), time.Duration(timeout)*time.Second)
	defer cancel()

	start := time.Now()
	results, err := c.Transact(ctx, operations...)
	elapsed := float64((time.Since(start)) / time.Millisecond)

	var dbType string
	switch c.Schema().Name {
	case "OVN_Northbound":
		dbType = "ovn-nb"
	}

	code := "0"
	defer func() {
		ovsClientRequestLatency.WithLabelValues(dbType, method, code).Observe(elapsed)
	}()

	if err != nil {
		code = "1"
		klog.Errorf("error occurred in transact with %s operations: %+v in %vms", dbType, operations, elapsed)
		return err
	}

	if elapsed > 500 {
		klog.Warningf("%s operations took too long: %+v in %vms", dbType, operations, elapsed)
	}

	errors, err := ovsdb.CheckOperationResults(results, operations)
	if err != nil {
		klog.Errorf("error occurred in transact with operations %+v with operation errors %+v: %v", operations, errors, err)
		return err
	}

	return nil
}
