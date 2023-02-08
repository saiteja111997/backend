package gcp

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"

	"cloud.google.com/go/storage"
	"github.com/gin-gonic/gin"
	"google.golang.org/api/option"
	"google.golang.org/appengine"
)

var (
	storageClient *storage.Client
)

func DownloadFromBucket(c *gin.Context, bucketName string, fileName string) []byte {
	bucket := bucketName
	fmt.Println("Downloading audio file from GCP...............")
	ctx := appengine.NewContext(c.Request)
	var err error
	storageClient, err = storage.NewClient(ctx, option.WithCredentialsFile("keys.json"))
	if err != nil {
		fmt.Println("Error 1 : ", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": err.Error(),
			"error":   true,
		})
		return nil
	}

	r, err := storageClient.Bucket(bucket).Object(fileName).NewReader(ctx)
	if err != nil {
		fmt.Println("Error 2 : ", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": err.Error(),
			"error":   true,
		})
		return nil
	}
	defer r.Close()

	data, err := ioutil.ReadAll(r)
	if err != nil {
		fmt.Println("Error 3 : ", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": err.Error(),
			"error":   true,
		})
		return nil
	}

	return data
}

func UploadToGoogleCloud(c *gin.Context, bucketName string, dataBytes []byte, fileName string) {
	bucket := bucketName
	ctx := appengine.NewContext(c.Request)
	var err error

	storageClient, err = storage.NewClient(ctx, option.WithCredentialsFile("keys.json"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": err.Error(),
			"error":   true,
		})
		return
	}

	sw := storageClient.Bucket(bucket).Object(fileName).NewWriter(ctx)

	if _, err := sw.Write(dataBytes); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": err.Error(),
			"error":   true,
		})
		return
	}

	if err := sw.Close(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": err.Error(),
			"error":   true,
		})
		return
	}

	u, err := url.Parse("/" + bucket + "/" + sw.Attrs().Name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": err.Error(),
			"Error":   true,
		})
		return
	}

	fmt.Println("Find the path of the file here : ", u.EscapedPath())
	c.Next()

	// c.JSON(http.StatusOK, gin.H{
	// 	"message":  "audio uploaded successfully",
	// 	"pathname": u.EscapedPath(),
	// })
}
