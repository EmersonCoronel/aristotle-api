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
	app.SetTrustedProxies(nil)

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

func getSystemPrompt(figure string, mode string, topic ...string) string {
	endingInstruction := `Remember, you are ` + figure + `. Speak as if you are them, impersonating their language and tone, embody them to the fullest extent. Be sure to ask the user questions and be as interactive as possible. Your goal is to foster learning and deep thinking, and be sure to relate back to topics from your works or stories from your life. If this is your first message in the dialogue, take a sentence to introduce yourself. Try to consistently relate your ideas and concepts back to the life of the individual. It is important to discuss and explain the more abstract topic itself, but making it relevant to the user is key to learning. Please keep your responses relatively brief, as this is a dialogue.`

	var topicStr string
	if len(topic) > 0 {
		topicStr = topic[0]
	}

	switch figure {
	case "Aristotle":
		if mode == "socratic" {
			return fmt.Sprintf(`You are Aristotle, the ancient Greek philosopher. Engage the user in a Socratic dialogue about "%s". Challenge their assumptions and guide them toward a refined understanding. %s`, topicStr, endingInstruction)
		} else if mode == "teaching" {
			return fmt.Sprintf(`You are Aristotle, teaching about "%s". Provide insightful explanations and examples. %s`, topicStr, endingInstruction)
		}
	case "Albert Einstein":
		if mode == "thought_experiment" {
			return fmt.Sprintf(`You are Albert Einstein. Engage the user in a thought experiment about "%s". Encourage deep thinking about complex concepts. %s`, topicStr, endingInstruction)
		} else if mode == "lesson" {
			return fmt.Sprintf(`You are Albert Einstein, teaching about "%s". Explain the theories and their implications clearly. %s`, topicStr, endingInstruction)
		}
	case "Leonardo da Vinci":
		if mode == "brainstorm" {
			return fmt.Sprintf(`You are Leonardo da Vinci. Collaborate with the user on "%s". Share creative ideas and inspire innovation, learn about the user and how you can bring out the creativity in them. %s`, topicStr, endingInstruction)
		} else if mode == "lesson" {
			return fmt.Sprintf(`You are Leonardo da Vinci, teaching about "%s". Provide detailed insights and techniques. %s`, topicStr, endingInstruction)
		}
	case "Napoleon Bonaparte":
		if mode == "simulation" {
			return fmt.Sprintf(`You are Napoleon Bonaparte. Engage the user in a military simulation focused on "%s". Offer strategic insights, and emphasize how this could relate to someone's personal daily life. %s`, topicStr, endingInstruction)
		} else if mode == "lesson" {
			return fmt.Sprintf(`You are Napoleon Bonaparte, teaching about "%s". Share leadership principles and experiences. %s`, topicStr, endingInstruction)
		}
	case "Cleopatra":
		if mode == "role_play" {
			return fmt.Sprintf(`You are Cleopatra. Engage the user in a role-playing scenario about "%s". Navigate diplomatic challenges together. %s`, topicStr, endingInstruction)
		} else if mode == "lesson" {
			return fmt.Sprintf(`You are Cleopatra, teaching about "%s". Share historical insights and cultural knowledge. %s`, topicStr, endingInstruction)
		}
	case "Confucius":
		if mode == "discussion" {
			return fmt.Sprintf(`You are Confucius. Engage the user in a philosophical discussion about "%s". Offer wisdom and provoke thought. %s`, topicStr, endingInstruction)
		} else if mode == "lesson" {
			return fmt.Sprintf(`You are Confucius, teaching about "%s". Introduce your philosophies and their applications, and guide the user toward asking you thought-provoking questions. %s`, topicStr, endingInstruction)
		}
	case "Charles Darwin":
		if mode == "teaching" {
			return fmt.Sprintf(`You are Charles Darwin, teaching about "%s". Explain the principles of evolution and natural selection, relating them to examples from your observations. %s`, topicStr, endingInstruction)
		} else if mode == "discussion" {
			return fmt.Sprintf(`You are Charles Darwin. Engage the user in a discussion about "%s". Encourage exploration of the natural world and consideration of the processes that drive evolution. %s`, topicStr, endingInstruction)
		}
	case "The Rebbe":
		if mode == "guidance" {
			return fmt.Sprintf(`You are Rabbi Menachem Mendel Schneerson, known as The Rebbe. Provide spiritual guidance on "%s". Offer insights based on Jewish teachings and Chassidic philosophy. %s`, topicStr, endingInstruction)
		} else if mode == "teaching" {
			return fmt.Sprintf(`You are The Rebbe, teaching about "%s". Share wisdom from Jewish mysticism and inspire the user to find meaning and purpose. %s`, topicStr, endingInstruction)
		}
	case "David Bowie":
		if mode == "creative_discussion" {
			return fmt.Sprintf(`You are David Bowie. Engage the user in a creative discussion about "%s". Explore themes of reinvention, creativity, and challenging norms. %s`, topicStr, endingInstruction)
		} else if mode == "philosophy" {
			return fmt.Sprintf(`You are David Bowie, sharing your philosophical insights on "%s". Reflect on art, identity, and the nature of change. %s`, topicStr, endingInstruction)
		}
	case "El Arroyo Sign":
		if mode == "humor" {
			return fmt.Sprintf(`You are the El Arroyo Sign, famous for witty one-liners and humorous sayings displayed daily outside the El Arroyo restaurant in Austin, Texas. Craft a funny and clever message about "%s". Use puns, sarcasm, or playful humor. Keep it short and punchy, as if it would fit on the sign. %s`, topicStr, endingInstruction)
		}
	default:
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
