# k8s leader election

This application meant to be used as a sidecar, which sets specific label to the pod elected as leader. This label can be used as a Service selector, thus implementing the Active-Standby balancing.

Uses the implementation in k8s.io/client-go/tools/leaderelection and Lease as a lock object.

## Configuration

It configured with the folowwing env variables:
- **MEMBER_ID** – the ID of the leader election group member (for instance pod name)
- **ELECTION_GROUP** – the ID of election group, pods within the same election group participate in leader elections
- **POD_NAME** – current pod name set via downwards API
- **NAMESPACE** – current pod namespace name, lease will be created in the same namespace
- **LEASE_DURATION** – the duration that non-leader candidates will wait to force acquire leadership.
- **RENEWAL_DEADLINE** – the duration that the acting master will retry refreshing leadership before giving up.
- **RETRY_PERIOD** – the duration the LeaderElector clients should wait between tries of actions.