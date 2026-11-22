// Paket connector menyediakan antarmuka generik untuk layanan luar yang
// diautentikasi lewat CLI yang sudah login di mesin pengguna.
package connector

import "context"

// State adalah keadaan sambungan sebuah connector.
type State string

const (
	// StateCLIMissing berarti CLI layanan belum terpasang.
	StateCLIMissing State = "cli_missing"
	// StateNotLoggedIn berarti CLI ada tetapi belum login.
	StateNotLoggedIn State = "not_logged_in"
	// StateConnected berarti CLI sudah login dan siap dipakai.
	StateConnected State = "connected"
)

// Status adalah hasil pengecekan sebuah connector. Tidak pernah memuat token.
type Status struct {
	State   State  `json:"state"`
	User    string `json:"user,omitempty"`
	Message string `json:"message,omitempty"`
}

// Guide adalah panduan menyambungkan connector.
type Guide struct {
	Steps    []string `json:"steps"`
	Commands []string `json:"commands"`
}

// Connector adalah satu layanan luar.
type Connector interface {
	ID() string
	Name() string
	Status(ctx context.Context) (Status, error)
	ConnectGuide() Guide
}
