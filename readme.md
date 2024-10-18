# Audit & Activity Log Library

## Description
The Audit & Activity Log Library is a Go package that utilizes RabbitMQ to manage audit and activity logs. This library provides functionalities for publishing and consuming both audit and activity logs efficiently.

## Prerequisites
- Go 1.16 or newer
- RabbitMQ
- Package `github.com/joho/godotenv`
- Package `github.com/rabbitmq/amqp091-go`
- Package `gorm.io/gorm` for database management

## Installation
1. **Install dependencies**:
   ```bash
   go get github.com/SAMBPLG/gologs
   ```

2. **Create a `.env` file** in the root directory with the following content:
   ```
   RABBITMQ_URL=amqp://user:password@localhost:5672/
   ```

   Replace `user`, `password`, and `localhost` with your RabbitMQ configuration.

## Usage

### 1. Publish Audit Log

Create a file named `publish_audit.go` with the following content:

```go
package main

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/joho/godotenv"
	gologs "github.com/SAMBPLG/gologs"
)

type User struct {
	Email string `json:"email"`
	Name  string `json:"name"`
}

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: Could not load .env file, using environment variables from the host")
	}

	if err := gologs.InitAuditLogClient(); err != nil {
		log.Fatalf("Failed to initialize RabbitMQ client: %v", err)
	}
	defer gologs.CloseGlobalClient()

	userBefore := User{Email: "user@example.com", Name: "Jane Doe"}
	userAfter := User{Email: "user@example.com", Name: "John Doe"}

	beforeJSON, _ := json.Marshal(userBefore)
	afterJSON, _ := json.Marshal(userAfter)

	for i := 1; i <= 5; i++ {
		auditLog := gologs.AuditLog{
			Module:     "UserModule",
			ActionType: "CREATE",
			SearchKey:  string(i),
			Before:     string(beforeJSON),
			After:      string(afterJSON),
			ActionBy:   "admin",
		}

		if err := gologs.PublishAuditLog(auditLog); err != nil {
			log.Printf("Failed to publish audit log: %v", err)
		} else {
			fmt.Printf("Published audit log: %+v\n", auditLog)
		}
	}

	fmt.Println("Finished publishing audit logs.")
}
```

Run the following command to publish the audit log:
```bash
go run publish_audit.go
```

### 2. Publish Activity Log

Create a file named `publish_activity.go` with the following content:

```go
package main

import (
	"fmt"
	"log"

	gologs "github.com/SAMBPLG/gologs"
	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: Could not load .env file, using environment variables from the host")
	}

	if err := gologs.InitActivityLogClient(); err != nil {
		log.Fatalf("Failed to initialize RabbitMQ client: %v", err)
	}
	defer gologs.CloseGlobalClient()

	for i := 1; i <= 5; i++ {
		activity := gologs.ActivityLog{
			UserID:     "uuid-1234-5678-90ab-cdef",
			Username:   "user@example.com",
			Action:     "LOGIN",
			Module:     "AuthModule",
			IPAddress:  "192.168.1.1",
			DeviceInfo: "Chrome on Windows",
			Remarks:    "User logged in successfully",
			Location:   "Jakarta",
		}

		if err := gologs.PublishActivityLog(activity); err != nil {
			log.Printf("Failed to publish activity log: %v", err)
		} else {
			fmt.Printf("Published activity log: %+v\n", activity)
		}
	}

	fmt.Println("Finished publishing activity logs.")
}
```

Run the following command to publish the activity log:
```bash
go run publish_activity.go
```

### 3. Consume Audit Log

Create a file named `consume_audit.go` with the following content:

```go
package main

import (
	"fmt"
	gologs "github.com/SAMBPLG/gologs"
	"log"
	"time"

	"github.com/joho/godotenv"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type AuditLogModel struct {
	gorm.Model
	Module      string `json:"module"`
	ActionType  string `json:"actionType"`
	SearchKey   string `json:"searchKey"`
	Before      string `json:"before"`
	After       string `json:"after"`
	ActionBy    string `json:"actionBy"`
	ActionTime  string `json:"timestamp"`
	CreatedTime string `json:"created_time"`
}

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: Could not load .env file, using environment variables from the host")
	}

	db, err := gorm.Open(sqlite.Open("gologs.db"), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to the database: %v", err)
	}

	if err := db.AutoMigrate(&AuditLogModel{}); err != nil {
		log.Fatalf("Failed to migrate database schema: %v", err)
	}

	if err := gologs.InitAuditLogClient(); err != nil {
		log.Fatalf("Failed to initialize RabbitMQ client: %v", err)
	}
	defer gologs.CloseGlobalClient()

	consumerName := "AuditLogConsumer1"
	prefetchCount := 50

	err = gologs.ConsumeAuditLogs(&consumerName, func(log gologs.AuditLog, ack func(bool)) {
		actionTime := log.ActionTime.Format("2006-01-02 15:04:05")
		createdTime := time.Now().Format("2006-01-02 15:04:05")

		auditLogModel := AuditLogModel{
			Module:      log.Module,
			ActionType:  log.ActionType,
			SearchKey:   log.SearchKey,
			Before:      log.Before,
			After:       log.After,
			ActionBy:    log.ActionBy,
			ActionTime:  actionTime,
			CreatedTime: createdTime,
		}

		if err := db.Create(&auditLogModel).Error; err != nil {
			fmt.Printf("Failed to save audit log to the database: %v\n", err)
			ack(false)
			return
		} else {
			fmt.Printf("Saved audit log to the database: %+v\n", auditLogModel)
			ack(true)
		}
	}, &prefetchCount)

	if err != nil {
		log.Fatalf("Failed to start consuming audit logs: %v", err)
	}

	select {}
}
```

Run the following command to consume audit logs:
```bash
go run consume_audit.go
```

### 4. Consume Activity Log

Create a file named `consume_activity.go` with the following content:

```go
package main

import (
	"fmt"
	gologs "github.com/SAMBPLG/gologs"
	"log"
	"time"

	"github.com/joho/godotenv"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type ActivityLogModel struct {
	gorm.Model
	UserID      string `json:"user_id"`
	Username    string `json:"username"`
	Action      string `json:"action"`
	Module      string `json:"module"`
	Timestamp   string `json:"timestamp"`
	IPAddress   string `json:"ip_address,omitempty"`
	DeviceInfo  string `json:"device_info,omitempty"`
	Location    string `json:"location,omitempty"`
	Remarks     string `json:"remarks,omitempty"`
	CreatedTime string `json:"created_time"`
}

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: Could not load .env file, using environment variables from the host")
	}

	db, err := gorm.Open(sqlite.Open("activitylogs.db"), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to the database: %v", err)
	}

	if err := db.AutoMigrate(&ActivityLogModel{}); err != nil {
		log.Fatalf("Failed to migrate database schema: %v", err)
	}

	if err := gologs.InitActivityLogClient(); err != nil {
		log.Fatalf("Failed to initialize RabbitMQ client: %v", err)
	}
	defer gologs.CloseGlobalClient()

	consumerName := "ActivityLogConsumer1"
	prefetchCount := 50

	err = gologs.ConsumeActivityLogs(&consumerName, func(log gologs.ActivityLog, ack func(bool)) {
		timestamp := log.ActionTime.Format("2006-01-02 15:04:05")
		createdTime := time.Now().Format("2006-01-02 15:04:05")

		activityLogModel := ActivityLogModel{
			UserID:      log.UserID,
			Username:    log.Username,
			Action:      log.Action,
			Module:      log.Module,
			Timestamp:   timestamp,
			IPAddress:   log.IPAddress,
			DeviceInfo:  log.DeviceInfo,
			Location:    log.Location,
			Remarks:     log.Remarks,
			CreatedTime: createdTime,
		}

		if err := db.Create(&activityLogModel).Error; err != nil {
			fmt.Printf("Failed to save activity log to the database: %v\n", err)
			ack(false)
			return
		} else {
			fmt.Printf("Saved activity log to the database: %+v\n", activityLogModel)
			ack(true)
		}
	}, &prefetchCount)

	if err != nil {
		log.Fatalf("Failed to start consuming activity logs: %v", err)
	}

	select {}
}
```

Run the following command to consume activity logs:
```bash
go run consume_activity.go
```

## Notes
- Ensure RabbitMQ is running when you execute this application.
- You can publish as many audit and activity logs as needed, and the consumers will receive those messages.