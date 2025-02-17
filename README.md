# Go Project Documentation

## Project Overview

This Go project is a VPN management system that integrates with Telegram bots, Outline VPN, and PocketBase for data storage and API interactions. It includes functionalities such as order processing, payment handling, and automated background tasks.

## Table of Contents

1. [Project Structure](#project-structure)
2. [Dependencies](#dependencies)
3. [Configuration](#configuration)
4. [Endpoints](#endpoints)
5. [Scheduled Tasks](#scheduled-tasks)
6. [Event Handlers](#event-handlers)
7. [Database Interactions](#database-interactions)
8. [Error Handling](#error-handling)
9. [Running the Application](#running-the-application)

---

## Project Structure

```
/go-project
│── main.go
│── config/          # Environment configurations
│── helpers/         # Utility functions
│── wrappers/        # API wrappers for external services
│   ├── outline/api/
│   ├── tg-bot/
│── pb_public/       # Static public files
│── views/           # HTML views
└── database/        # Database models and queries
```

## Dependencies

The project relies on several external dependencies:

- `github.com/labstack/echo/v5` - Web framework for API routing
- `github.com/pocketbase/pocketbase` - Lightweight backend for database operations
- `github.com/robfig/cron/v3` - Task scheduler for background jobs
- `github.com/pocketbase/dbx` - Database query builder

## Configuration

The project uses environment variables stored in a `.env` file or a configuration module:

```go
tgbotWebhookServer := env.Get("TELEGRAM_WEBHOOK_URL")
```

## Endpoints

### Static File Handling

- `GET /*` - Serves static files from `pb_public` directory.

### Pricing Page

- `GET /pricing/:name` - Renders pricing page using HTML templates.

### VPN Configuration Download

- `GET /ssconf/:conf_id` - Retrieves VPN configuration and provides it as a CSV file.

## Scheduled Tasks

Cron jobs are used for automated background processing:

- `"30 * * * *"` - Synchronizes VPN usage data.
- `"*/1 * * * *"` - Checks server health and notifies Telegram admins.

## Event Handlers

### Order Creation

Triggers actions when a new order is created:

- Updates order status to `INCOMPLETE`.
- Generates payment records.
- Handles free-tier VPN access.

### Order Approval

Triggered when an order approval record is created:

- Updates the order status to `WAIT_FOR_APPROVE`.
- Sends Telegram notifications to admins.

### Order Completion

When an order is marked as `COMPLETE`, the system:

- Allocates VPN resources.
- Assigns servers based on capacity.
- Sends configuration details to the user.

### VPN Configuration Deletion

Before deleting a VPN configuration:

- Revokes Outline VPN access keys.
- Updates server and plan capacities.

## Database Interactions

Uses PocketBase ORM for querying and updating records:

```go
app.Dao().FindRecordById("vpn_configs", conf_id)
```

## Error Handling

Errors are logged and returned where applicable:

```go
if err != nil {
    log.Print("Error retrieving plan: ", err)
    return err
}
```

## Running the Application

To start the server:

```sh
go run main.go
```

