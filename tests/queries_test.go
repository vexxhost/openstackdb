//go:build integration

package tests

import (
	"context"
	"github.com/stretchr/testify/require"
	cinder "github.com/vexxhost/openstackdb/cinder/db"
	glance "github.com/vexxhost/openstackdb/glance/db"
	heat "github.com/vexxhost/openstackdb/heat/db"
	"github.com/vexxhost/openstackdb/internal/testutil"
	ironic "github.com/vexxhost/openstackdb/ironic/db"
	keystone "github.com/vexxhost/openstackdb/keystone/db"
	magnum "github.com/vexxhost/openstackdb/magnum/db"
	manila "github.com/vexxhost/openstackdb/manila/db"
	neutron "github.com/vexxhost/openstackdb/neutron/db"
	nova_api "github.com/vexxhost/openstackdb/nova/db/api"
	nova "github.com/vexxhost/openstackdb/nova/db/main"
	octavia "github.com/vexxhost/openstackdb/octavia/db/repositories"
	placement "github.com/vexxhost/openstackdb/placement/objects"
	"testing"
)

func TestNovaQueries(t *testing.T) {
	ctx := context.Background()
	conn := testutil.NewMySQLContainer(t, "nova", "../sql/nova/schema.sql", "../sql/nova/indexes.sql")
	q := nova.New(conn)
	var err error
	_, err = q.InstanceGetAll(ctx)
	require.NoError(t, err, "InstanceGetAll")
	_, err = q.ServiceGetAll(ctx)
	require.NoError(t, err, "ServiceGetAll")
	_, err = q.ComputeNodeGetAll(ctx)
	require.NoError(t, err, "ComputeNodeGetAll")
}
func TestNovaApiQueries(t *testing.T) {
	ctx := context.Background()
	conn := testutil.NewMySQLContainer(t, "nova_api", "../sql/nova_api/schema.sql", "../sql/nova_api/indexes.sql")
	q := nova_api.New(conn)
	var err error
	_, err = q.FlavorGetAll(ctx)
	require.NoError(t, err, "FlavorGetAll")
	_, err = q.QuotaGetAll(ctx)
	require.NoError(t, err, "QuotaGetAll")
	_, err = q.AggregateGetAll(ctx)
	require.NoError(t, err, "AggregateGetAll")
	_, err = q.AggregateHostGetAll(ctx)
	require.NoError(t, err, "AggregateHostGetAll")
	_, err = q.QuotaClassGetDefaults(ctx)
	require.NoError(t, err, "QuotaClassGetDefaults")
	_, err = q.QuotaUsageGetAll(ctx)
	require.NoError(t, err, "QuotaUsageGetAll")
}
func TestCinderQueries(t *testing.T) {
	ctx := context.Background()
	conn := testutil.NewMySQLContainer(t, "cinder", "../sql/cinder/schema.sql", "../sql/cinder/indexes.sql")
	q := cinder.New(conn)
	var err error
	_, err = q.ServiceGetAll(ctx)
	require.NoError(t, err, "ServiceGetAll")
	_, err = q.QuotaGetProjectLimits(ctx)
	require.NoError(t, err, "QuotaGetProjectLimits")
	_, err = q.QuotaUsageGetAll(ctx)
	require.NoError(t, err, "QuotaUsageGetAll")
	_, err = q.QuotaGetAllWithUsage(ctx)
	require.NoError(t, err, "QuotaGetAllWithUsage")
	_, err = q.VolumeTypeGetAll(ctx)
	require.NoError(t, err, "VolumeTypeGetAll")
	_, err = q.SnapshotCount(ctx)
	require.NoError(t, err, "SnapshotCount")
	// No service_uuid index is present: NULL and populated values both match.
	testutil.SeedSQL(t, conn,
		`INSERT INTO volume_types(id,name,deleted) VALUES ('type','fast',0)`,
		`INSERT INTO volumes(id,volume_type_id,service_uuid,deleted) VALUES ('active-null','type',NULL,0),('active-service','type','svc',0),('deleted','type','svc',1)`,
		`INSERT INTO volume_attachment(id,volume_id,instance_uuid,deleted) VALUES ('a','active-service','server',0),('b','active-service','old-server',1)`,
	)
	volumes, err := q.VolumeGetAllWithAttachments(ctx)
	require.NoError(t, err)
	require.Len(t, volumes, 2)
	byID := make(map[string]cinder.VolumeGetAllWithAttachmentsRow)
	for _, volume := range volumes {
		byID[volume.ID] = volume
	}
	require.Contains(t, byID, "active-null")
	require.Contains(t, byID, "active-service")
	require.Equal(t, "fast", byID["active-null"].VolumeType.String)
	require.False(t, byID["active-null"].ServerID.Valid)
	require.Equal(t, "server", byID["active-service"].ServerID.String)
	_, err = q.VolumeGetAllWithAttachments(ctx)
	require.NoError(t, err, "VolumeGetAllWithAttachments")
	_, err = q.VolumeGetAll(ctx)
	require.NoError(t, err, "VolumeGetAll")
}
func TestGlanceQueries(t *testing.T) {
	ctx := context.Background()
	conn := testutil.NewMySQLContainer(t, "glance", "../sql/glance/schema.sql")
	q := glance.New(conn)
	var err error
	_, err = q.ImageGetAll(ctx)
	require.NoError(t, err, "ImageGetAll")
}
func TestKeystoneQueries(t *testing.T) {
	ctx := context.Background()
	conn := testutil.NewMySQLContainer(t, "keystone", "../sql/keystone/schema.sql")
	q := keystone.New(conn)
	var err error
	_, err = q.ListProjects(ctx)
	require.NoError(t, err, "ListProjects")
	_, err = q.ListDomains(ctx)
	require.NoError(t, err, "ListDomains")
	_, err = q.ListUsers(ctx)
	require.NoError(t, err, "ListUsers")
	_, err = q.ListRegions(ctx)
	require.NoError(t, err, "ListRegions")
	_, err = q.ListGroups(ctx)
	require.NoError(t, err, "ListGroups")
}
func TestHeatQueries(t *testing.T) {
	ctx := context.Background()
	conn := testutil.NewMySQLContainer(t, "heat", "../sql/heat/schema.sql")
	q := heat.New(conn)
	var err error
	_, err = q.StackGetAll(ctx)
	require.NoError(t, err, "StackGetAll")
}
func TestIronicQueries(t *testing.T) {
	ctx := context.Background()
	conn := testutil.NewMySQLContainer(t, "ironic", "../sql/ironic/schema.sql")
	q := ironic.New(conn)
	var err error
	_, err = q.GetNodeList(ctx)
	require.NoError(t, err, "GetNodeList")
}
func TestMagnumQueries(t *testing.T) {
	ctx := context.Background()
	conn := testutil.NewMySQLContainer(t, "magnum", "../sql/magnum/schema.sql")
	q := magnum.New(conn)
	var err error
	_, err = q.GetClusterList(ctx)
	require.NoError(t, err, "GetClusterList")
}
func TestManilaQueries(t *testing.T) {
	ctx := context.Background()
	conn := testutil.NewMySQLContainer(t, "manila", "../sql/manila/prereqs.sql", "../sql/manila/schema.sql")
	q := manila.New(conn)
	var err error
	_, err = q.ShareGetAllWithInstances(ctx)
	require.NoError(t, err, "ShareGetAllWithInstances")
}
func TestNeutronQueries(t *testing.T) {
	ctx := context.Background()
	conn := testutil.NewMySQLContainer(t, "neutron", "../sql/neutron/schema.sql")
	q := neutron.New(conn)
	var err error
	_, err = q.GetAgents(ctx)
	require.NoError(t, err, "GetAgents")
	_, err = q.GetHARouterAgentPortBindingsWithAgents(ctx)
	require.NoError(t, err, "GetHARouterAgentPortBindingsWithAgents")
	_, err = q.GetRouters(ctx)
	require.NoError(t, err, "GetRouters")
	_, err = q.GetFloatingIPs(ctx)
	require.NoError(t, err, "GetFloatingIPs")
	_, err = q.GetNetworks(ctx)
	require.NoError(t, err, "GetNetworks")
	_, err = q.GetSubnets(ctx)
	require.NoError(t, err, "GetSubnets")
	_, err = q.GetPorts(ctx)
	require.NoError(t, err, "GetPorts")
	_, err = q.GetSecurityGroupCount(ctx)
	require.NoError(t, err, "GetSecurityGroupCount")
	_, err = q.GetNetworkIPAvailabilitiesUsed(ctx)
	require.NoError(t, err, "GetNetworkIPAvailabilitiesUsed")
	_, err = q.GetNetworkIPAvailabilitiesTotal(ctx)
	require.NoError(t, err, "GetNetworkIPAvailabilitiesTotal")
	_, err = q.GetSubnetPools(ctx)
	require.NoError(t, err, "GetSubnetPools")
	_, err = q.GetQuotas(ctx)
	require.NoError(t, err, "GetQuotas")
	_, err = q.GetResourceCountsByProject(ctx)
	require.NoError(t, err, "GetResourceCountsByProject")
}
func TestOctaviaQueries(t *testing.T) {
	ctx := context.Background()
	conn := testutil.NewMySQLContainer(t, "octavia", "../sql/octavia/schema.sql")
	q := octavia.New(conn)
	var err error
	_, err = q.PoolGetAll(ctx)
	require.NoError(t, err, "PoolGetAll")
	_, err = q.LoadBalancerGetAllWithVIP(ctx)
	require.NoError(t, err, "LoadBalancerGetAllWithVIP")
	_, err = q.AmphoraGetAll(ctx)
	require.NoError(t, err, "AmphoraGetAll")
}
func TestPlacementQueries(t *testing.T) {
	ctx := context.Background()
	conn := testutil.NewMySQLContainer(t, "placement", "../sql/placement/schema.sql")
	q := placement.New(conn)
	var err error
	_, err = q.GetResourceProviderInventories(ctx)
	require.NoError(t, err, "GetResourceProviderInventories")
	_, err = q.GetAllocationsByProject(ctx)
	require.NoError(t, err, "GetAllocationsByProject")
	_, err = q.GetConsumerCountByProject(ctx)
	require.NoError(t, err, "GetConsumerCountByProject")
	_, err = q.GetResourceClasses(ctx)
	require.NoError(t, err, "GetResourceClasses")
	_, err = q.GetConsumers(ctx)
	require.NoError(t, err, "GetConsumers")
}
