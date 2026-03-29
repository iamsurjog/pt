package scripts

import "os"


func GetVenvPath() string {
	 return os.Getenv("VIRTUAL_ENV")
}
