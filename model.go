package model

var (
	STORAGE      = ""
	STORAGE_PATH = ""

	USERNAME = ""
	PASSWORD = ""
)

func Start(storageApi, storagePath, username, password string) {
	STORAGE = storageApi
	STORAGE_PATH = storagePath
	USERNAME = username
	PASSWORD = password
}
