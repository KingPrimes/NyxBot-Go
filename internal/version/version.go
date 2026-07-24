package version

var (
	Version     = "dev"
	ProductName = "NyxBot"
	Description = "NyxBot Server"
	Commit      = "unknown"
)

func String() string {
	return Version
}
