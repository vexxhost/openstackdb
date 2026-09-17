# API mapping

The initial extraction preserves the exporter's selected columns, joins,
aggregations, default deletion predicates, and index hints. Method names follow
upstream resource terminology; signatures and return projections are Go-specific.
This table is an exporter migration map, not a claim that every operation exists
with identical semantics upstream.

| Package | Former exporter query | New method |
| --- | --- | --- |
| `nova/db/main` | `GetInstances` | `InstanceGetAll` |
| `nova/db/main` | `GetServices` | `ServiceGetAll` |
| `nova/db/main` | `GetComputeNodes` | `ComputeNodeGetAll` |
| `nova/db/api` | `GetFlavors` | `FlavorGetAll` |
| `nova/db/api` | `GetQuotas` | `QuotaGetAll` |
| `nova/db/api` | `GetAggregates` | `AggregateGetAll` |
| `nova/db/api` | `GetAggregateHosts` | `AggregateHostGetAll` |
| `nova/db/api` | `GetQuotaClassDefaults` | `QuotaClassGetDefaults` |
| `nova/db/api` | `GetQuotaUsages` | `QuotaUsageGetAll` |
| `cinder/db` | `GetAllServices` | `ServiceGetAll` |
| `cinder/db` | `GetProjectQuotaLimits` | `QuotaGetProjectLimits` |
| `cinder/db` | `GetProjectQuotaUsages` | `QuotaUsageGetAll` |
| `cinder/db` | `GetAllProjectQuotas` | `QuotaGetAllWithUsage` |
| `cinder/db` | `GetVolumeTypes` | `VolumeTypeGetAll` |
| `cinder/db` | `GetSnapshotCount` | `SnapshotCount` |
| `cinder/db` | `GetAllVolumes` | `VolumeGetAllWithAttachments` |
| `glance/db` | `GetAllImages` | `ImageGetAll` |
| `heat/db` | `GetStackMetrics` | `StackGetAll` |
| `ironic/db` | `GetNodeMetrics` | `GetNodeList` |
| `keystone/db` | `GetProjectMetrics` | `ListProjects` |
| `keystone/db` | `GetDomainMetrics` | `ListDomains` |
| `keystone/db` | `GetUserMetrics` | `ListUsers` |
| `keystone/db` | `GetRegionMetrics` | `ListRegions` |
| `keystone/db` | `GetGroupMetrics` | `ListGroups` |
| `magnum/db` | `GetClusterMetrics` | `GetClusterList` |
| `manila/db` | `GetShareMetrics` | `ShareGetAllWithInstances` |
| `neutron/db` | `GetAgents` | `GetAgents` |
| `neutron/db` | `GetHARouterAgentPortBindingsWithAgents` | `GetHARouterAgentPortBindingsWithAgents` |
| `neutron/db` | `GetRouters` | `GetRouters` |
| `neutron/db` | `GetFloatingIPs` | `GetFloatingIPs` |
| `neutron/db` | `GetNetworks` | `GetNetworks` |
| `neutron/db` | `GetSubnets` | `GetSubnets` |
| `neutron/db` | `GetPorts` | `GetPorts` |
| `neutron/db` | `GetSecurityGroupCount` | `GetSecurityGroupCount` |
| `neutron/db` | `GetNetworkIPAvailabilitiesUsed` | `GetNetworkIPAvailabilitiesUsed` |
| `neutron/db` | `GetNetworkIPAvailabilitiesTotal` | `GetNetworkIPAvailabilitiesTotal` |
| `neutron/db` | `GetSubnetPools` | `GetSubnetPools` |
| `neutron/db` | `GetQuotas` | `GetQuotas` |
| `neutron/db` | `GetResourceCountsByProject` | `GetResourceCountsByProject` |
| `octavia/db/repositories` | `GetAllPools` | `PoolGetAll` |
| `octavia/db/repositories` | `GetAllLoadBalancersWithVip` | `LoadBalancerGetAllWithVIP` |
| `octavia/db/repositories` | `GetAllAmphora` | `AmphoraGetAll` |
| `placement/objects` | `GetResourceMetrics` | `GetResourceProviderInventories` |
| `placement/objects` | `GetAllocationsByProject` | `GetAllocationsByProject` |
| `placement/objects` | `GetConsumerCountByProject` | `GetConsumerCountByProject` |
| `placement/objects` | `GetResourceClasses` | `GetResourceClasses` |
| `placement/objects` | `GetConsumers` | `GetConsumers` |

## Upstream references

- [Nova main database API](https://github.com/openstack/nova/blob/master/nova/db/main/api.py): `instance_get_by_uuid`, `instance_get_all_by_filters`, `service_get_all`, `compute_node_get_all`.
- [Nova API database](https://github.com/openstack/nova/tree/master/nova/db/api) and [objects](https://github.com/openstack/nova/tree/master/nova/objects): flavor, aggregate, and quota data.
- [Cinder DB API](https://github.com/openstack/cinder/blob/master/cinder/db/api.py): `volume_get`, `volume_get_all`, `volume_type_get_all`, `service_get_all`.
- [Glance DB API](https://github.com/openstack/glance/blob/master/glance/db/sqlalchemy/api.py): `image_get`, `image_get_all`.
- [Ironic DB API](https://github.com/openstack/ironic/blob/master/ironic/db/api.py) and [Magnum DB API](https://github.com/openstack/magnum/blob/master/magnum/db/api.py): `get_node_list`, `get_cluster_list`.
- [Heat DB API](https://github.com/openstack/heat/blob/master/heat/db/api.py): `stack_get_all`.
- [Manila DB API](https://github.com/openstack/manila/blob/master/manila/db/api.py): share operations; the instance join is an explicit extension.
- [Keystone backends](https://github.com/openstack/keystone/tree/master/keystone): `list_*` resource, identity, and catalog operations are grouped into a single read projection package here.
- [Neutron database plugins](https://github.com/openstack/neutron/tree/master/neutron/db): `get_*` resource operations; capacity/count queries are extensions.
- [Octavia repositories](https://github.com/openstack/octavia/blob/master/octavia/db/repositories.py): resource repositories with `get_all` operations.
- [Placement objects](https://github.com/openstack/placement/tree/master/placement/objects): inventory, allocation, and consumer projections.

## Deliberate differences

- Nova `ServiceGetAll` retains the exporter's requirement that `last_seen_up` and `topic` are non-null.
- The filtered APIs support exact membership and lifetime overlap, not every upstream regex, pagination, eager-loading, or authorization option.
- Current exporter queries retain their existing deleted/hidden/status predicates; they are not full upstream administrative listings.
- Errors are Go/database errors; absent single resources return `sql.ErrNoRows` rather than Python service-specific exceptions.
- Aggregate queries such as `QuotaGetAllWithUsage`, `GetResourceProviderInventories`, and `GetResourceCountsByProject` are explicitly library extensions.
- No cross-cell routing, Nova archive union, or historic schema negotiation is performed.
