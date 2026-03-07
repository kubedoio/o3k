package database

import (
	"time"
)

// Keystone models
type User struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	PasswordHash string    `json:"-"`
	Enabled      bool      `json:"enabled"`
	DomainID     string    `json:"domain_id"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type Project struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Enabled     bool      `json:"enabled"`
	DomainID    string    `json:"domain_id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Domain struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Enabled     bool      `json:"enabled"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Role struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

type RoleAssignment struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	ProjectID string    `json:"project_id"`
	RoleID    string    `json:"role_id"`
	CreatedAt time.Time `json:"created_at"`
}

// Nova models
type Flavor struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	VCPUs     int       `json:"vcpus"`
	RAMMB     int       `json:"ram"`
	DiskGB    int       `json:"disk"`
	IsPublic  bool      `json:"OS-FLV-EXT-DATA:ephemeral"`
	CreatedAt time.Time `json:"created_at"`
}

type Instance struct {
	ID              string     `json:"id"`
	Name            string     `json:"name"`
	ProjectID       string     `json:"tenant_id"`
	UserID          string     `json:"user_id"`
	FlavorID        string     `json:"flavor_id"`
	ImageID         string     `json:"image_id"`
	Status          string     `json:"status"`
	PowerState      int        `json:"OS-EXT-STS:power_state"`
	LibvirtDomainID string     `json:"-"`
	CreatedAt       time.Time  `json:"created"`
	UpdatedAt       time.Time  `json:"updated"`
	LaunchedAt      *time.Time `json:"OS-SRV-USG:launched_at"`
	TerminatedAt    *time.Time `json:"OS-SRV-USG:terminated_at"`
}

type Keypair struct {
	ID          string    `json:"-"`
	Name        string    `json:"name"`
	UserID      string    `json:"user_id"`
	PublicKey   string    `json:"public_key"`
	Fingerprint string    `json:"fingerprint"`
	CreatedAt   time.Time `json:"created_at"`
}

// Neutron models
type Network struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	ProjectID    string    `json:"tenant_id"`
	AdminStateUp bool      `json:"admin_state_up"`
	Status       string    `json:"status"`
	Shared       bool      `json:"shared"`
	MTU          int       `json:"mtu"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type Subnet struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	NetworkID      string    `json:"network_id"`
	ProjectID      string    `json:"tenant_id"`
	CIDR           string    `json:"cidr"`
	GatewayIP      string    `json:"gateway_ip"`
	IPVersion      int       `json:"ip_version"`
	EnableDHCP     bool      `json:"enable_dhcp"`
	DNSNameservers []string  `json:"dns_nameservers"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type Port struct {
	ID           string                   `json:"id"`
	Name         string                   `json:"name"`
	NetworkID    string                   `json:"network_id"`
	ProjectID    string                   `json:"tenant_id"`
	DeviceID     string                   `json:"device_id"`
	DeviceOwner  string                   `json:"device_owner"`
	MACAddress   string                   `json:"mac_address"`
	AdminStateUp bool                     `json:"admin_state_up"`
	Status       string                   `json:"status"`
	FixedIPs     []map[string]interface{} `json:"fixed_ips"`
	CreatedAt    time.Time                `json:"created_at"`
	UpdatedAt    time.Time                `json:"updated_at"`
}

type SecurityGroup struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	ProjectID   string    `json:"tenant_id"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type SecurityGroupRule struct {
	ID              string    `json:"id"`
	SecurityGroupID string    `json:"security_group_id"`
	Direction       string    `json:"direction"`
	EtherType       string    `json:"ethertype"`
	Protocol        string    `json:"protocol"`
	PortRangeMin    int       `json:"port_range_min"`
	PortRangeMax    int       `json:"port_range_max"`
	RemoteIPPrefix  string    `json:"remote_ip_prefix"`
	RemoteGroupID   string    `json:"remote_group_id"`
	CreatedAt       time.Time `json:"created_at"`
}

// Cinder models
type Volume struct {
	ID                   string     `json:"id"`
	Name                 string     `json:"name"`
	ProjectID            string     `json:"tenant_id"`
	UserID               string     `json:"user_id"`
	SizeGB               int        `json:"size"`
	Status               string     `json:"status"`
	Bootable             bool       `json:"bootable"`
	AttachedToInstanceID *string    `json:"attachments"`
	RBDPool              string     `json:"-"`
	RBDImage             string     `json:"-"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
}

type VolumeType struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	IsPublic    bool      `json:"is_public"`
	CreatedAt   time.Time `json:"created_at"`
}

// Glance models
type Image struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	ProjectID       *string   `json:"owner"`
	Status          string    `json:"status"`
	Visibility      string    `json:"visibility"`
	SizeBytes       int64     `json:"size"`
	DiskFormat      string    `json:"disk_format"`
	ContainerFormat string    `json:"container_format"`
	MinDiskGB       int       `json:"min_disk"`
	MinRAMMB        int       `json:"min_ram"`
	Checksum        string    `json:"checksum"`
	RBDPool         string    `json:"-"`
	RBDImage        string    `json:"-"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}
