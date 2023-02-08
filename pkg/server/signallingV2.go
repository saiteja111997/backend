package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/saiteja111997/videoChatApp/pkg/gcp"
)

// AllRooms is the global hashmap for the server
var AllRooms RoomMap

// var videoBuffer []byte
var audioBuffer []byte

type summaryRequest struct {
	AudioUrl      string `json:"audio_url"`
	Summarization bool   `json:"summarization"`
	SummaryModel  string `json:"summary_model"`
	SummaryType   string `json:"summary_type"`
	SpeakerLables bool   `json:"speaker_labels"`
}

// Enable CORS to set the header as origin
func EnableCors(c *gin.Context) {
	c.Header("Access-Control-Allow-Origin", "http://localhost:3000")
	// c.Header("Access-Control-Allow-Origin", "https://projectai-ba364.web.app")
}

// THIS FUNCTION CREATES AN AUDIO FILE ON THE DISC LOCALLY
// func createOpusFile(audioBytes []byte) {
// 	// Create a new file for the audio data
// 	file, err := os.Create("audio.opus")
// 	if err != nil {
// 		log.Fatal(err)
// 	}
// 	// Write the audio bytes to the file
// 	_, err = file.Write(audioBytes)
// 	if err != nil {
// 		log.Fatal(err)
// 	}

// 	// Close the file
// 	err = file.Close()
// 	if err != nil {
// 		log.Fatal(err)
// 	}
// }

// CreateRoomRequestHandler Create a Room and return roomID
func CreateRoomRequestHandler(c *gin.Context) {
	AllRooms.InitiateRoom()
	log.Println("The initialized map is : ", AllRooms.Map)
	EnableCors(c)
	roomID := AllRooms.CreateRoom()
	type resp struct {
		RoomID string `json:"room_id"`
	}
	c.JSON(200, resp{RoomID: roomID})
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type broadcastMsg struct {
	Message map[string]interface{}
	RoomID  string
	Client  *websocket.Conn
}

var broadcast = make(chan broadcastMsg)

func broadcaster() {
	for {
		msg := <-broadcast
		AllRooms.Mutex.Lock()
		for _, client := range AllRooms.Map[msg.RoomID] {
			if client.Conn != msg.Client {
				err := client.Conn.WriteJSON(msg.Message)
				if err != nil {
					log.Printf("Connection has been closed due to the following reason : %s", err)
					fmt.Println("Closing the websocket connection!!")
					client.Conn.Close()
				}
			}
		}
		AllRooms.Mutex.Unlock()
	}
}

// JoinRoomRequestHandler will join the client in a particular room
func JoinRoomRequestHandler(c *gin.Context) {
	roomID := c.Query("roomID")
	if roomID == "" {
		log.Println("roomID missing in URL Parameters")
		return
	}

	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Fatal("Web Socket Upgrade Error", err)
	}
	defer ws.Close()

	AllRooms.InsertIntoRoom(roomID, false, ws)
	count := 0

	go broadcaster()

	for {
		var msg broadcastMsg

		_, message, err := ws.ReadMessage()
		if err != nil {
			log.Fatal(err)
		}
		err = json.Unmarshal(message, &msg.Message)
		if err != nil {
			fmt.Println("This is a stream message!!")
			fmt.Println("The length of the buffer is : ", len(message))
			audioBuffer = append(audioBuffer, message...)
		} else {
			if msg.Message["stopRecoridng"] != nil {
				// createWavFile(audioBuffer)
				gcp.UploadToGoogleCloud(c, "audio_data_bucket_1", audioBuffer, "output.opus")
			}

			msg.Client = ws
			msg.RoomID = roomID
			log.Println("Msg id : ", count)
			log.Println("Here is a message : ", msg.Message)
			count++

			broadcast <- msg
		}
	}
}

func TranscribeAudioHandler(c *gin.Context) {

	audioBytes := gcp.DownloadFromBucket(c, "audio_data_bucket_1", "output.opus")

	// createOpusFile(audioBytes)

	ch := make(chan string)

	go getUploadUrl(audioBytes, ch)

	// fmt.Println("Here is the text : ", transcription)

	url := <-ch

	fmt.Println("Here is the url : ", url)

	textBytes, err := json.Marshal(url)

	if err != nil {
		log.Fatal(err.Error())
	}

	gcp.UploadToGoogleCloud(c, "audio_data_bucket_1", textBytes, "urls.txt")

	endpoint := "https://api.assemblyai.com/v2/transcript"

	payload := map[string]string{"audio_url": url}
	body, err := json.Marshal(payload)
	if err != nil {
		fmt.Println(err)
		return
	}

	client := &http.Client{}
	req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(body))
	if err != nil {
		fmt.Println(err)
		return
	}

	assemblyAiApiKey := os.Getenv("ASSEMBLY_AI_API_KEY")
	fmt.Println(assemblyAiApiKey)

	req.Header.Set("Authorization", assemblyAiApiKey)
	req.Header.Set("Content-Type", "application/json")

	res, err := client.Do(req.WithContext(context.Background()))
	if err != nil {
		return
	}
	defer res.Body.Close()

	body, err = ioutil.ReadAll(res.Body)
	if err != nil {
		return
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return
	}

	polling_id := result["id"].(string)
	poll_status := result["status"].(string)
	var poll_result map[string]interface{}

	fmt.Println("Here is the initial Status : ", poll_status)

	for {
		if poll_status == "completed" {
			break
		}

		time.Sleep(2 * time.Second)
		client := &http.Client{}
		req, err := http.NewRequest("GET", "https://api.assemblyai.com/v2/transcript/"+polling_id, nil)
		if err != nil {
			fmt.Println("failed to create a new request!!")
			return
		}
		req.Header.Set("Authorization", assemblyAiApiKey)
		res, err := client.Do(req.WithContext(context.Background()))
		if err != nil {
			return
		}
		defer res.Body.Close()

		body, err := ioutil.ReadAll(res.Body)
		if err != nil {
			return
		}

		if err := json.Unmarshal(body, &poll_result); err != nil {
			return
		}
		if poll_result["status"] != nil {
			poll_status = poll_result["status"].(string)
		}

		fmt.Println("Here is the updated poll result : ", poll_result)
	}

	fmt.Println("The final result : ", poll_result)

	c.JSON(http.StatusOK, gin.H{
		"message": "Audio Transcribed Successfully",
		"Text":    poll_result["text"].(string),
	})
}

func getUploadUrl(audioBytes []byte, ch chan string) {

	buf := bytes.NewBuffer(audioBytes)

	client := &http.Client{}
	req, err := http.NewRequest("POST", "https://api.assemblyai.com/v2/upload", buf)
	if err != nil {
		return
	}

	assemblyAiApiKey := os.Getenv("ASSEMBLY_AI_API_KEY")
	fmt.Println(assemblyAiApiKey)

	req.Header.Set("Authorization", assemblyAiApiKey)
	req.Header.Set("Content-Type", "application/octet-stream")

	res, err := client.Do(req.WithContext(context.Background()))
	if err != nil {
		return
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return
	}

	ch <- result["upload_url"].(string)
}

func GenerateFinalResult(c *gin.Context) {

	audioUrl := gcp.DownloadFromBucket(c, "audio_data_bucket_1", "urls.txt")
	var url string
	err := json.Unmarshal(audioUrl, &url)

	if err != nil {
		log.Fatal(err.Error())
		return
	}

	fmt.Println("Here is the downloaded url : ", url)

	endpoint := "https://api.assemblyai.com/v2/transcript"
	jsonBody := summaryRequest{}

	jsonBody.AudioUrl = url
	jsonBody.Summarization = true
	jsonBody.SpeakerLables = true
	jsonBody.SummaryModel = "conversational"
	jsonBody.SummaryType = "bullets"

	body, err := json.Marshal(jsonBody)

	if err != nil {
		log.Fatal(err.Error())
		return
	}

	buf := bytes.NewBuffer(body)

	client := &http.Client{}
	req, err := http.NewRequest("POST", endpoint, buf)
	if err != nil {
		log.Fatal(err.Error())
		return
	}

	assemblyAiApiKey := os.Getenv("ASSEMBLY_AI_API_KEY")
	fmt.Println(assemblyAiApiKey)

	req.Header.Set("Authorization", assemblyAiApiKey)
	req.Header.Set("Content-Type", "application/json")

	res, err := client.Do(req.WithContext(context.Background()))
	if err != nil {
		return
	}
	defer res.Body.Close()

	body, err = ioutil.ReadAll(res.Body)
	if err != nil {
		return
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Final Result",
		"Result":  result,
	})

}

// func GenerateFinalResult(c *gin.Context) {

// 	text := c.Request.PostFormValue("transcript")

// 	prompt := text + "Get updates of Siddharth for the current and next sprints based on the below conversation."

// openAiApiKey := os.Getenv("OPEN_AI_API_KEY")
// 	fmt.Println(openAiApiKey)

// 	client := gogpt.NewClient(openAiApiKey)
// 	ctx := context.Background()

// 	req := gogpt.CompletionRequest{
// 		Model:     gogpt.GPT3Ada,
// 		MaxTokens: 100,
// 		Prompt:    prompt,
// 	}
// 	resp, err := client.CreateCompletion(ctx, req)
// 	if err != nil {
// 		log.Fatal(err.Error())
// 	}
// 	fmt.Println(resp.Choices[0].Text)

// 	c.JSON(http.StatusOK, gin.H{
// 		"message": "Your tasks!!",
// 		"Tasks":   resp.Choices[0].Text,
// 	})
// }

// const (
// 	apiURL = "https://api.openai.com/v1/engines/text-davinci-001/jobs"
// 	model  = "text-davinci-002"
// )

// func GenerateFinalResultV2(c *gin.Context) {

// 	prompt := c.Request.PostFormValue("transcript")

// 	reqBody := map[string]interface{}{
// 		"model":             model,
// 		"prompt":            prompt,
// 		"temperature":       0.5,
// 		"max_tokens":        100,
// 		"top_p":             1,
// 		"frequency_penalty": 0,
// 		"presence_penalty":  0,
// 	}

// 	reqBytes, err := json.Marshal(reqBody)
// 	if err != nil {
// 		fmt.Println(err)
// 		return
// 	}

// 	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(reqBytes))
// 	if err != nil {
// 		fmt.Println(err)
// 		return
// 	}

// openAiApiKey := os.Getenv("OPEN_AI_API_KEY")
// 	fmt.Println(openAiApiKey)

// 	req.Header.Add("Content-Type", "application/json")
// 	req.Header.Add("Authorization", "Bearer " + openAiApiKey)

// 	client := http.Client{}
// 	res, err := client.Do(req)
// 	if err != nil {
// 		fmt.Println(err)
// 		return
// 	}
// 	defer res.Body.Close()

// 	resBody, err := ioutil.ReadAll(res.Body)
// 	if err != nil {
// 		fmt.Println(err)
// 		return
// 	}

// 	var response map[string]interface{}
// 	err = json.Unmarshal(resBody, &response)
// 	if err != nil {
// 		fmt.Println(err)
// 		return
// 	}
// 	fmt.Println("Print response : ", response)

// 	fmt.Println(response["choices"].([]interface{})[0].(map[string]interface{})["text"])

// 	c.JSON(http.StatusOK, gin.H{
// 		"message": "Convo summary!!",
// 		"Tasks":   response["choices"].([]interface{})[0].(map[string]interface{})["text"],
// 	})

// }
