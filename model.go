package model

var (
	StorageOrigin     string
	StoragePathOrigin string
	Username          string
	Password          string
)

func Config(storageOrigin, storagePathOrigin, username, password string) {
	StorageOrigin = storageOrigin
	StoragePathOrigin = storagePathOrigin
	Username = username
	Password = password

}

func Start() {

}
