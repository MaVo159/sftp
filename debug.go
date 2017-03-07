// +build debug

package sftp

// import "log"
import "fmt"

func debug(format string, args ...interface{}) {
	// log.Printf(format, args...)
	fmt.Printf(format, args...)
	fmt.Println()
}
