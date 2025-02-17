# Go Project Documentation

## Project Overview
## Table of Contents

- [Overview](#overview)
- [Features](#features)
- [Requirements](#requirements)
- [Installation](#installation)
  - [Clone the Repository](#1-clone-the-repository)
  - [Set Up Environment Variables](#2-set-up-environment-variables)
  - [Install Dependencies](#3-install-dependencies)
  - [Build the Application](#4-build-the-application)
  - [Start the Application](#5-start-the-application)
- [Running Migrations](#running-migrations)
  - [Manual Migrations (CLI)](#manual-migrations-cli)
  - [Automatic Migrations in Code](#automatic-migrations-in-code)
- [Adding a New Server](#adding-a-new-server)
  - [Requirements](#requirements-1)
  - [Setting up Outline VPN](#setting-up-outline-vpn)
    - [On CentOS](#on-centos)
- [Usage](#usage)
- [Contributing](#contributing)
- [License](#license)

---

## Overview

This project sets up and manages VPN infrastructure, with a specific focus on **Outline VPN** configuration and server management. It utilizes the **PocketBase** backend and integrates with services like **Telegram Bot** for notifications. The backend is built in **Go**, and the server management includes cron jobs, API integrations, and dynamic configuration handling.

---

## Features

- **Server Health Monitoring**: Regular checks for active servers and sending notifications to admins.
- **Outline VPN Management**: Integration with Outline VPN API for creating and managing access keys.
- **Cron Jobs**: Scheduling regular tasks such as VPN config syncing and server health checks.
- **Payment System**: Manages orders, plans, and payments, integrating with payment gateways.
- **Dynamic Config Generation**: Automatic generation of VPN config files (CSV) for user download.
- **Webhooks**: Sends server health data and order updates via Telegram Bot.

---

## Requirements

1. **Operating System**:
   - Ubuntu (Recommended: 18.04 or newer)
   - CentOS (Recommended: 7 or newer)
   
2. **Go Version**: 1.18 or newer
3. **Docker & Kubernetes** (for deployment)
4. **PocketBase**: Used as the backend database system

---

## Installation

### 1. Clone the Repository

```bash
git clone https://github.com/yourusername/projectname.git
cd projectname
```

### 2. Set Up Environment Variables

Ensure you have the necessary environment variables configured in a `.env` file:

```bash
TELEGRAM_WEBHOOK_URL=<your-telegram-webhook-url>
DB_HOST=<database-host>
DB_PORT=<database-port>
DB_USER=<database-user>
DB_PASSWORD=<database-password>
```

### 3. Install Dependencies

Run the following command to install required dependencies:

```bash
go mod tidy
```

### 4. Build the Application

To build the application:

```bash
go build -o app
```

### 5. Start the Application

Start the application using:

```bash
./app
```

---

## Running Migrations

To run database migrations, you can use the built-in PocketBase migrations system.

### Manual Migrations (CLI)

1. **Navigate to the project directory:**
   ```bash
   cd /path/to/your/project
   ```

2. **Run the migrations:**
   ```bash
   ./pb migrate
   ```

### Automatic Migrations in Code

Integrate migration checks directly into your Go application, so they run automatically on app startup:

```go
app.OnBeforeServe().Add(func(e *core.ServeEvent) error {
    if err := app.Migrate(); err != nil {
        return fmt.Errorf("failed to run migrations: %w", err)
    }
    return nil
})
```

---

## Adding a New Server

To set up a new server for your VPN infrastructure, you should ensure that the server is running either **Ubuntu** or **CentOS**. Both operating systems are supported for running the required VPN software.

### Requirements

1. **Operating System**: 
   - Ubuntu (Recommended: 18.04 or newer)
   - CentOS (Recommended: 7 or newer)
   
2. **Server Configuration**: The server should meet the minimum hardware requirements for hosting the VPN service:
   - At least 1 GB RAM (2 GB recommended)
   - 1 vCPU (2 or more recommended)
   - A stable internet connection

### Setting up Outline VPN

For **Ubuntu** and **CentOS** systems, you will need to install and configure the Outline VPN server.

#### On **CentOS**:

1. **Install `wget`**:
   First, you need to ensure that `wget` is installed on your CentOS server.

   ```bash
   sudo yum install wget -y
   ```

2. **Install Outline VPN**:
   - Visit the **Outline Manager** to get the installation script.
   - Copy the installation script and paste it on your server:

   ```bash
   wget https://outlinevpn.github.io/install.sh
   sudo bash install.sh
   ```

   - Follow the instructions provided by Outline Manager to complete the installation.

---

## Usage

Explain how to use the application once it's up and running. Provide examples for interacting with the API or accessing key features.

---

## Contributing

We welcome contributions! Please fork the repository, make changes, and submit a pull request.

---

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
```
