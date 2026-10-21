package main

// Tenant definitions live here so main.go stays about loading

type tenantSpec struct {
	Name     string
	Seed     uint64
	Machines int
	Products int
	Rows     int // inspection_results rows
	Users    int // managers, each with one live session
	Currency string
	Domain   string // email domain for the tenants's users
}

var specs = []tenantSpec{
	{Name: "Ironvale Castings", Seed: 1, Machines: 40, Products: 120, Rows: 10_000_000, Users: 200, Currency: "USD", Domain: "ironvale.example"},
	{Name: "Brightline Plastics", Seed: 2, Machines: 15, Products: 60, Rows: 2_700_000, Users: 100, Currency: "EUR", Domain: "brightline.example"},
	{Name: "Kestrel Fasteners", Seed: 3, Machines: 6, Products: 25, Rows: 10_000, Users: 50, Currency: "GBP", Domain: "kestrel.example"},
}
