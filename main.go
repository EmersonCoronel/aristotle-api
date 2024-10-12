package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	openai "github.com/sashabaranov/go-openai"
)

// Load environment variables from .env file
func init() {
	err := godotenv.Load()
	if err != nil {
		fmt.Println("No .env file found or error reading .env file")
	}
}

// Message represents a chat message
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
	Name    string `json:"name,omitempty"`
}

// ChatRequestBody represents the request body for /api/chat
type ChatRequestBody struct {
	Message        string    `json:"message"`
	Messages       []Message `json:"messages"`
	Mode           string    `json:"mode"`
	SelectedFigure string    `json:"selectedFigure"`
	SelectedTopic  string    `json:"selectedTopic,omitempty"`
}

// StartDialogueRequestBody represents the request body for /api/start-dialogue
type StartDialogueRequestBody struct {
	Figure string `json:"figure"`
	Mode   string `json:"mode"`
	Topic  string `json:"topic"`
}

func main() {
	app := gin.Default()

	// Define CORS options
	corsConfig := cors.Config{
		AllowOrigins:     []string{"http://localhost:3000", "https://emersoncoronel.com"},
		AllowMethods:     []string{"GET", "POST", "OPTIONS"},
		AllowHeaders:     []string{"Content-Type", "Authorization"},
		AllowCredentials: true,
	}

	app.Use(cors.New(corsConfig))

	openaiAPIKey := os.Getenv("OPENAI_API_KEY")
	if openaiAPIKey == "" {
		fmt.Println("OPENAI_API_KEY environment variable not set")
		os.Exit(1)
	}

	client := openai.NewClient(openaiAPIKey)

	// Chat endpoint
	app.POST("/api/chat", func(c *gin.Context) {
		var reqBody ChatRequestBody
		if err := c.ShouldBindJSON(&reqBody); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
			return
		}

		fmt.Println("Received message:", reqBody.Message)
		fmt.Println("Mode:", reqBody.Mode)
		fmt.Println("Figure:", reqBody.SelectedFigure)
		fmt.Println("Topic:", reqBody.SelectedTopic)

		systemPrompt := getSystemPrompt(reqBody.SelectedFigure, reqBody.Mode, reqBody.SelectedTopic)

		// Convert client messages to OpenAI messages
		var messages []openai.ChatCompletionMessage
		messages = append(messages, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleSystem,
			Content: systemPrompt,
		})

		for _, msg := range reqBody.Messages {
			messages = append(messages, openai.ChatCompletionMessage{
				Role:    msg.Role,
				Content: msg.Content,
			})
		}

		// Set headers to enable SSE
		c.Writer.Header().Set("Content-Type", "text/event-stream")
		c.Writer.Header().Set("Cache-Control", "no-cache")
		c.Writer.Header().Set("Connection", "keep-alive")
		c.Writer.Flush()

		ctx := c.Request.Context()

		req := openai.ChatCompletionRequest{
			Model:    "gpt-3.5-turbo",
			Messages: messages,
			Stream:   true,
		}

		stream, err := client.CreateChatCompletionStream(ctx, req)
		if err != nil {
			fmt.Println("Error creating stream:", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error creating stream"})
			return
		}
		defer stream.Close()

		// Handle streaming response
		for {
			response, err := stream.Recv()
			if err != nil {
				fmt.Println("Error receiving stream:", err)
				break
			}

			if len(response.Choices) > 0 {
				content := response.Choices[0].Delta.Content
				if content != "" {
					data := fmt.Sprintf("data: %s\n\n", jsonString(content))
					c.Writer.Write([]byte(data))
					c.Writer.Flush()
					time.Sleep(100 * time.Millisecond) // Artificial delay
				}
			}
		}

		c.Writer.Write([]byte("data: [DONE]\n\n"))
		c.Writer.Flush()
	})

	// Start Dialogue Endpoint
	app.POST("/api/start-dialogue", func(c *gin.Context) {
		var reqBody StartDialogueRequestBody
		if err := c.ShouldBindJSON(&reqBody); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
			return
		}

		fmt.Printf("Starting dialogue with %s in mode %s on topic %s\n", reqBody.Figure, reqBody.Mode, reqBody.Topic)

		systemPrompt := getSystemPrompt(reqBody.Figure, reqBody.Mode, reqBody.Topic)

		messages := []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: systemPrompt,
			},
		}

		// Set headers to enable SSE
		c.Writer.Header().Set("Content-Type", "text/event-stream")
		c.Writer.Header().Set("Cache-Control", "no-cache")
		c.Writer.Header().Set("Connection", "keep-alive")
		c.Writer.Flush()

		ctx := c.Request.Context()

		req := openai.ChatCompletionRequest{
			Model:    "gpt-3.5-turbo",
			Messages: messages,
			Stream:   true,
		}

		stream, err := client.CreateChatCompletionStream(ctx, req)
		if err != nil {
			fmt.Println("Error creating stream:", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error creating stream"})
			return
		}
		defer stream.Close()

		// Handle streaming response
		for {
			response, err := stream.Recv()
			if err != nil {
				fmt.Println("Error receiving stream:", err)
				break
			}

			if len(response.Choices) > 0 {
				content := response.Choices[0].Delta.Content
				if content != "" {
					data := fmt.Sprintf("data: %s\n\n", jsonString(content))
					c.Writer.Write([]byte(data))
					c.Writer.Flush()
					time.Sleep(100 * time.Millisecond) // Artificial delay
				}
			}
		}

		c.Writer.Write([]byte("data: [DONE]\n\n"))
		c.Writer.Flush()
	})

	// Start the server
	port := os.Getenv("PORT")
	if port == "" {
		port = "4000"
	}
	app.Run(":" + port)
}

// Function to generate system prompts based on figure, mode, and topic
func getSystemPrompt(figure, mode, topic string) string {
	endingInstruction := fmt.Sprintf(`Remember, you are %s. Speak as if you are them, impersonating their language and tone, embody them to the fullest extent. Be sure to ask the user questions and be as interactive as possible. Your goal is to foster learning and deep thinking, and be sure to relate back to topics from your works or stories from your life. If this is your first message in the dialogue, take a sentence to introduce yourself. Try to consistently relate your ideas and concepts back to the life of the individual. It is important to discuss and explain the more abstract topic itself, but making it relevant to the user is key to learning. Please keep your responses relatively brief, as this is a dialogue.`, figure)

	switch figure {
	case "Aristotle":
		if mode == "socratic" {
			return fmt.Sprintf(`You are Aristotle, the ancient Greek philosopher. Engage the user in a Socratic dialogue about "%s". Challenge their assumptions and guide them toward a refined understanding. %s`, topic, endingInstruction)
		} else if mode == "teaching" {
			return fmt.Sprintf(`You are Aristotle, teaching about "%s". Provide insightful explanations and examples. %s`, topic, endingInstruction)
		}
	// Add other cases for different figures and modes here...

	default:
		// Scenario-Based Advice for any figure
		if mode == "scenario" {
			return fmt.Sprintf(`You are %s, offering advice based on your expertise and experiences. Provide thoughtful guidance to the user's situation or question. %s`, figure, endingInstruction)
		} else {
			return fmt.Sprintf(`You are %s. Engage in a meaningful conversation with the user. %s`, figure, endingInstruction)
		}
	}
	return endingInstruction
}

// Helper function to JSON-encode a string
func jsonString(str string) string {
	b, _ := json.Marshal(str)
	return string(b)
}
