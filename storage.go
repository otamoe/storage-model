package model

import (
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/globalsign/mgo"
	"github.com/globalsign/mgo/bson"
	"github.com/otamoe/gin-server/errs"
	mgoModel "github.com/otamoe/mgo-model"
	"github.com/sirupsen/logrus"
)

type (
	Storage struct {
		mgoModel.DocumentBase `json:"-" bson:"-" binding:"-"`
		ID                    bson.ObjectId `json:"_id" bson:"_id" binding:"required,objectid"`

		Unique string `json:"unique" bson:"unique" binding:"required"`

		Path string `json:"path" bson:"path" binding:"required"`

		HLS    string `json:"hls,omitempty" bson:"hls,omitempty"`
		HLSKey string `json:"hls_key,omitempty" bson:"hls_key,omitempty"`

		Status  string `json:"status,omitempty" bson:"status" binding:"required,oneof=pending approved unapproved banned"`
		Name    string `json:"name,omitempty" bson:"name" binding:"omitempty,max=512"`
		Type    string `json:"type,omitempty" bson:"type" binding:"omitempty,max=32"`
		SubType string `json:"sub_type,omitempty" bson:"sub_type" binding:"omitempty,max=64"`

		Size     int64                  `json:"size,omitempty" bson:"size" binding:"omitempty,min=0"`
		Duration float64                `json:"duration,omitempty" bson:"duration,omitempty" binding:"omitempty,min=0,max=2592000"`
		Width    int                    `json:"width,omitempty" bson:"width,omitempty" binding:"omitempty,min=0,max=32767"`
		Height   int                    `json:"height,omitempty" bson:"height,omitempty" binding:"omitempty,min=0,max=32767"`
		Pixels   int                    `json:"pixels,omitempty" bson:"pixels,omitempty" binding:"omitempty,min=0,max=268435456"`
		Meta     map[string]interface{} `json:"meta,omitempty" bson:"meta,omitempty"`

		Complete bool `json:"complete,omitempty" bson:"complete"`

		CreatedAt *time.Time `json:"created_at,omitempty" bson:"created_at" binding:"required"`
		UpdatedAt *time.Time `json:"updated_at,omitempty" bson:"updated_at" binding:"required"`
		DeletedAt *time.Time `json:"deleted_at,omitempty" bson:"deleted_at,omitempty"`

		Errors     []*errs.Error `json:"errors,omitempty" bson:"errors,omitempty"`
		StatusCode int           `json:"status_code,omitempty" bson:"status_code,omitempty"`
	}
)

var (
	ErrStorageNotFound error = &errs.Error{
		Message:    "File not found",
		Path:       "storage",
		Type:       "not_found",
		StatusCode: http.StatusNotFound,
	}

	ModelStorage = &mgoModel.Model{
		Name:     "storages",
		Document: &Storage{},
		Indexs: []mgo.Index{
			mgo.Index{
				Key:        []string{"unique"},
				Unique:     true,
				Background: true,
			},
		},
	}
)

func Get(ctx context.Context, val string, cache bool, save bool) (storage *Storage, err error) {
	val2 := strings.Split(val, "/")
	var url string
	var auth bool
	if len(val2) == 2 && bson.IsObjectIdHex(val2[0]) && bson.IsObjectIdHex(val2[1]) {
		if StorageOrigin == "" {
			err = errors.New("storage-model.StorageOrigin is required")
			return
		}
		url = StorageOrigin + "/" + val + "/"
	} else {
		if StoragePathOrigin == "" {
			err = ErrStorageNotFound
			return
		}
		url = StoragePathOrigin + "/" + val
		auth = true
		for _, val := range val2 {
			if val == "" || strings.TrimSpace(val) != val || val[0] == '.' || strings.ContainsAny(val, "/:*?#%&<>\\") {
				err = ErrStorageNotFound
				return
			}
		}
	}
	storage = &Storage{}
	if cache {
		if err = ModelStorage.Query(ctx).Eq("unique", val).One(storage); err != mgo.ErrNotFound {
			if len(storage.Errors) != 0 {
				err = storage.Errors[0]
			}
			return
		}
	}
	err = nil
	if storage.Unique == "" {
		storage = fetch(url, auth)
		storage.Unique = val
	}

	if save {
		var isNew bool
		if cache {
			isNew = true
		} else if err = ModelStorage.Query(ctx).Eq("unique", val).One(storage); err == mgo.ErrNotFound {
			isNew = true
		}
		if isNew {
			storage.ID = bson.NewObjectId()
		}
		storage.New(ctx, ModelStorage, storage, isNew)
	}
	if len(storage.Errors) != 0 {
		err = storage.Errors[0]
	}
	return
}

func fetch(url string, auth bool) (storage *Storage) {

	var err error
	storage = &Storage{}

	defer func() {
		if err == nil {
			return
		}
		var ginErr *errs.Error
		switch err.(type) {
		case *errs.Error:
			ginErr = err.(*errs.Error)
			if ginErr.Err != nil {
				ginErr.Message = ginErr.Err.Error()
				ginErr.Err = nil
			}
		default:
			ginErr = &errs.Error{
				Message: err.Error(),
			}
		}

		storage.Errors = append(storage.Errors, ginErr)
		if storage.StatusCode != 0 {

		} else if ginErr.StatusCode != 0 {
			storage.StatusCode = ginErr.StatusCode
		} else {
			storage.StatusCode = http.StatusInternalServerError
		}
	}()
	var res *http.Response
	var bodyBytes []byte

	client := &http.Client{}

	timeoutCtx, timeoutCancel := context.WithTimeout(context.Background(), time.Second*20)
	defer timeoutCancel()

	var req *http.Request
	if req, err = http.NewRequest("GET", url, nil); err != nil {
		err = ErrStorageNotFound
		return
	}
	if auth {
		if Username == "" {
			err = errors.New("storage-model.Username is required")
			return
		}
		if Password == "" {
			err = errors.New("storage-model.Password is required")
			return
		}
		req.SetBasicAuth(Username, Password)
	}
	req = req.WithContext(timeoutCtx)
	if res, err = client.Do(req); err != nil {
		return
	}
	defer res.Body.Close()
	if bodyBytes, err = ioutil.ReadAll(res.Body); err != nil {
		return
	}

	logrus.Debugf("[Storage] %d %s", res.StatusCode, string(bodyBytes))

	if res.StatusCode >= 500 {
		err = &errs.Error{
			Message:    "Storage: Status code error",
			StatusCode: res.StatusCode,
		}
		return
	}
	if res.StatusCode > 200 {
		err = ErrStorageNotFound
		return
	}

	if err = json.Unmarshal(bodyBytes, storage); err != nil {
		return
	}
	return
}
