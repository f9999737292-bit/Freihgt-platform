package controltower

import "testing"

func TestCanAccessControlTower(t *testing.T) {
	tests := []struct {
		name  string
		roles []string
		want  bool
	}{
		{name: "platform admin", roles: []string{"PLATFORM_ADMIN"}, want: true},
		{name: "carrier dispatcher", roles: []string{"CARRIER_DISPATCHER"}, want: true},
		{name: "shipper admin", roles: []string{"SHIPPER_ADMIN"}, want: true},
		{name: "shipper logist", roles: []string{"SHIPPER_LOGIST"}, want: true},
		{name: "forwarder manager", roles: []string{"FORWARDER_MANAGER"}, want: true},
		{name: "finance manager denied", roles: []string{"FINANCE_MANAGER"}, want: false},
		{name: "procurement manager denied", roles: []string{"PROCUREMENT_MANAGER"}, want: false},
		{name: "unknown role denied", roles: []string{"UNKNOWN_ROLE"}, want: false},
		{name: "no roles denied", roles: nil, want: false},
		{name: "mixed allowed role", roles: []string{"FINANCE_MANAGER", "SHIPPER_LOGIST"}, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CanAccessControlTower(tt.roles); got != tt.want {
				t.Fatalf("CanAccessControlTower(%v) = %v, want %v", tt.roles, got, tt.want)
			}
		})
	}
}
