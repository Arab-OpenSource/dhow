package proto_test

import (
	"testing"

	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"

	agentv1 "github.com/Arab-OpenSource/dhow/packages/proto/gen/go/dhow/agent/v1"
	runnerv1 "github.com/Arab-OpenSource/dhow/packages/proto/gen/go/dhow/runner/v1"
)

const (
	runnerPackage = protoreflect.FullName("dhow.runner.v1")
	agentPackage  = protoreflect.FullName("dhow.agent.v1")
)

var withoutLease = map[protoreflect.FullName]bool{
	"dhow.runner.v1.RegistrationService.Register": true,
	"dhow.runner.v1.AuthService.Challenge":        true,
	"dhow.runner.v1.AuthService.Authenticate":     true,
	"dhow.runner.v1.AuthService.RotateKeys":       true,
	"dhow.runner.v1.LeaseService.Lease":           true,
}

func files(t *testing.T, pkg protoreflect.FullName) []protoreflect.FileDescriptor {
	t.Helper()
	var out []protoreflect.FileDescriptor
	protoregistry.GlobalFiles.RangeFilesByPackage(pkg, func(fd protoreflect.FileDescriptor) bool {
		out = append(out, fd)
		return true
	})
	if len(out) == 0 {
		t.Fatalf("no files registered for package %s", pkg)
	}
	return out
}

func methods(t *testing.T) []protoreflect.MethodDescriptor {
	t.Helper()
	var out []protoreflect.MethodDescriptor
	for _, pkg := range []protoreflect.FullName{runnerPackage, agentPackage} {
		for _, fd := range files(t, pkg) {
			for i := range fd.Services().Len() {
				svc := fd.Services().Get(i)
				for j := range svc.Methods().Len() {
					out = append(out, svc.Methods().Get(j))
				}
			}
		}
	}
	return out
}

func messages(t *testing.T, pkg protoreflect.FullName) []protoreflect.MessageDescriptor {
	t.Helper()
	var out []protoreflect.MessageDescriptor
	var walk func(protoreflect.MessageDescriptors)
	walk = func(mds protoreflect.MessageDescriptors) {
		for i := range mds.Len() {
			out = append(out, mds.Get(i))
			walk(mds.Get(i).Messages())
		}
	}
	for _, fd := range files(t, pkg) {
		walk(fd.Messages())
	}
	return out
}

func TestRegisteredPackages(t *testing.T) {
	tests := []struct {
		name string
		file protoreflect.FileDescriptor
		pkg  protoreflect.FullName
	}{
		{"runner", runnerv1.File_dhow_runner_v1_lease_proto, runnerPackage},
		{"agent", agentv1.File_dhow_agent_v1_agent_proto, agentPackage},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.file.Package() != tc.pkg {
				t.Fatalf("got package %s, want %s", tc.file.Package(), tc.pkg)
			}
		})
	}
}

func TestNoStreamingMethods(t *testing.T) {
	all := methods(t)
	if len(all) < 11 {
		t.Fatalf("found %d methods, the runner protocol declares at least 11", len(all))
	}
	for _, m := range all {
		if m.IsStreamingClient() || m.IsStreamingServer() {
			t.Errorf("%s is a streaming method: the runner protocol is unary only (ADR-0042)", m.FullName())
		}
	}
}

func TestMutatingRequestsCarryLeaseRef(t *testing.T) {
	leaseRef := (&runnerv1.LeaseRef{}).ProtoReflect().Descriptor().FullName()
	seen := map[protoreflect.FullName]bool{}
	for _, m := range methods(t) {
		seen[m.FullName()] = true
		if withoutLease[m.FullName()] {
			continue
		}
		fields := m.Input().Fields()
		found := false
		for i := range fields.Len() {
			f := fields.Get(i)
			if f.Kind() == protoreflect.MessageKind && f.Cardinality() != protoreflect.Repeated &&
				f.Message().FullName() == leaseRef {
				found = true
			}
		}
		if !found {
			t.Errorf("%s changes lease state, so %s needs a %s field", m.FullName(), m.Input().FullName(), leaseRef)
		}
	}
	for name := range withoutLease {
		if !seen[name] {
			t.Errorf("%s is exempt from LeaseRef but is not declared", name)
		}
	}
}

func TestLeaseGrantTravelsAsSignedBytes(t *testing.T) {
	grant := (&runnerv1.LeaseGrant{}).ProtoReflect().Descriptor().FullName()
	for _, md := range messages(t, runnerPackage) {
		fields := md.Fields()
		for i := range fields.Len() {
			f := fields.Get(i)
			if f.Kind() == protoreflect.MessageKind && f.Message().FullName() == grant {
				t.Errorf("%s embeds %s: a grant only travels as signed bytes", f.FullName(), grant)
			}
		}
	}
	signed := (&runnerv1.SignedLeaseGrant{}).ProtoReflect().Descriptor().Fields()
	tests := []struct {
		field protoreflect.Name
		kind  protoreflect.Kind
	}{
		{"grant_bytes", protoreflect.BytesKind},
		{"signature", protoreflect.BytesKind},
		{"key_version", protoreflect.Int32Kind},
	}
	for _, tc := range tests {
		t.Run(string(tc.field), func(t *testing.T) {
			f := signed.ByName(tc.field)
			if f == nil || f.Kind() != tc.kind {
				t.Fatalf("SignedLeaseGrant.%s must be %s", tc.field, tc.kind)
			}
		})
	}
}
