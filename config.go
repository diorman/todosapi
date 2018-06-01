package todospoc

var Config = struct {
	MainSQLDBSource  string
	KongAdminAddress string
	JWTSecret        string
}{
	"postgres://user:pass@main-database/db?sslmode=disable",
	"http://kong:8001",
	"super-secrer-key-for-jwt",
	// "postgres://user:pass@leads-database/db?sslmode=disable"
}
