# Recast

A Discord bot template built with Go, featuring a robust architecture and modern tooling. This template provides a solid foundation for building scalable Discord bots with best practices in mind.

## Features

- Discord bot integration with sharding support for handling large numbers of servers
- PostgreSQL database integration with automatic migrations
- Redis caching layer for improved performance
- Environment-based configuration for flexible deployment
- Docker deployment support for containerized environments
- Structured logging with slog
- Transaction support for database operations
- Soft delete functionality built-in

## Prerequisites

- Go 1.24 or later
- PostgreSQL (version 12 or later recommended)
- Redis (version 6 or later recommended)
- Docker (optional, for containerized deployment)
- Discord Bot Token (obtain from [Discord Developer Portal](https://discord.com/developers/applications))

## Configuration

The application is configured through environment variables. Create a `.env` file in the root directory with the following variables:

- `DEBUG`: Enable debug mode (true/false)
- `BOT_TOKEN`: Your Discord bot token from the Discord Developer Portal
- `BOT_INTENTS`: Discord bot intents (default: 32509)
  - This default includes: Guilds, Guild Messages, Direct Messages, Message Content
- `DATABASE_URL`: PostgreSQL connection URL (format: postgres://user:password@host:port/dbname)
- `REDIS_URL`: Redis connection URL (format: redis://user:password@host:port/db)
- `SHARD_ID`: Bot shard ID (default: 0)
- `SHARD_COUNT`: Total number of shards (default: 1)

## Development

1. Clone the repository:
   ```bash
   git clone https://github.com/yourusername/recast.git
   cd recast
   ```

2. Install dependencies:
   ```bash
   go mod download
   ```

3. Set up your environment variables:
   ```bash
   cp .env.example .env
   # Edit .env with your configuration
   ```

4. Start PostgreSQL and Redis:
   ```bash
   # Using Docker (if you have Docker installed)
   docker run -d --name postgres -e POSTGRES_PASSWORD=postgres -p 5432:5432 postgres:latest
   docker run -d --name redis -p 6379:6379 redis:latest
   ```

5. Run the application:
   ```bash
   go run cmd/recast/main.go
   ```

## Database

The project uses PostgreSQL as its primary database with automatic migration support. The database package (`internal/database/database.go`) automatically runs migrations on initialization, ensuring your database schema is always up to date.

Key database features:
- Automatic migration execution on application startup
- Connection pooling with optimized settings
- Transaction support for atomic operations
- Soft delete functionality (records are marked as deleted rather than removed)
- Built-in audit fields (created, updated, deleted timestamps)

To manually run migrations:
```bash
migrate -database "${DATABASE_URL}" -path migrations up
```

To create a new migration:
```bash
migrate create -ext sql -dir migrations -seq your_migration_name
```

## Deployment

The project includes Docker support in the `deployments` directory. Build and run using:

```bash
# Build the Docker image
docker build -t recast .

# Run the container
docker run -d \
  -e BOT_TOKEN=your_token \
  -e DATABASE_URL=your_db_url \
  -e REDIS_URL=your_redis_url \
  --name recast-bot \
  recast
```

For production deployment, consider:
- Using Docker Compose for managing multiple services
- Setting up proper logging and monitoring
- Implementing health checks
- Using secrets management for sensitive data
- Setting up proper backup strategies for the database

## Project Structure

- `cmd/`: Application entry points
  - `recast/`: Main application entry point
- `internal/`: Private application code
  - `bot/`: Discord bot implementation
  - `database/`: Database connection and operations
  - `models/`: Data models and database schema
- `migrations/`: Database migration files
- `deployments/`: Deployment configurations
- `scripts/`: Utility scripts

## Implementing Handlers

The bot uses a handler-based architecture for processing Discord events. Here's how to implement your own handlers:

### Command Structure

Create a new file in `internal/handlers/commands/` for your command:

```go
package commands

import (
    "context"
    "fmt"

    dg "github.com/bwmarrin/discordgo"
    "github.com/glotchimo/recast/internal/handlers"
    rp "github.com/glotchimo/recast/internal/response"
)

type MyCommand struct{}

func (c *MyCommand) Metadata() dg.ApplicationCommand {
    return dg.ApplicationCommand{
        Name:        "mycommand",
        Description: "Description of what your command does",
    }
}

func (c *MyCommand) Handle(ctx context.Context, dep handlers.Dependencies) error {
    if err := dep.Responder.Defer(dep.Interaction, true); err != nil {
        return err
    }

    embed := dg.MessageEmbed{
        Title:       "Response Title",
        Description: "Your response here",
    }

    return dep.Responder.Send(dep.Interaction, rp.MessageOptions{
        Embeds:    []*dg.MessageEmbed{&embed},
        Ephemeral: true,
    })
}
```

### Command Dependencies

The `handlers.Dependencies` struct provides access to common resources:

```go
type Dependencies struct {
    Session     *discordgo.Session
    Database    *database.Database
    Cache       *cache.Cache
    Responder   *response.Responder
    Logger      *slog.Logger
    Guild       *models.Guild
    Interaction *discordgo.InteractionCreate
    Options     *map[string]*discordgo.ApplicationCommandInteractionDataOption
}
```

### Using Database in Commands

Here's an example of a command that uses the database:

```go
package commands

import (
    "context"
    "fmt"

    dg "github.com/bwmarrin/discordgo"
    "github.com/glotchimo/recast/internal/handlers"
    rp "github.com/glotchimo/recast/internal/response"
)

type UserStats struct{}

func (c *UserStats) Metadata() dg.ApplicationCommand {
    return dg.ApplicationCommand{
        Name:        "stats",
        Description: "View user statistics",
    }
}

func (c *UserStats) Handle(ctx context.Context, dep handlers.Dependencies) error {
    dep.Responder.Defer(dep.Interaction, true)

    // Use the database
    user, err := dep.Database.GetUser(ctx, dep.Interaction.Member.User.ID)
    if err != nil {
        return fmt.Errorf("failed to get user: %w", err)
    }

    embed := dg.MessageEmbed{
        Title:       "User Statistics",
        Description: fmt.Sprintf("Messages: %d", user.MessageCount),
    }

    return dep.Responder.Send(dep.Interaction, rp.MessageOptions{
        Embeds:    []*dg.MessageEmbed{&embed},
        Ephemeral: true,
    })
}
```

### Command Options

For commands that require user input, you can define options in the metadata:

```go
func (c *MyCommand) Metadata() dg.ApplicationCommand {
    return dg.ApplicationCommand{
        Name:        "echo",
        Description: "Echo a message",
        Options: []*dg.ApplicationCommandOption{
            {
                Type:        dg.ApplicationCommandOptionString,
                Name:        "message",
                Description: "The message to echo",
                Required:    true,
            },
        },
    }
}

func (c *MyCommand) Handle(ctx context.Context, dep handlers.Dependencies) error {
    dep.Responder.Defer(dep.Interaction, true)

    // Get the message option
    message := dep.Interaction.ApplicationCommandData().Options[0].StringValue()

    embed := dg.MessageEmbed{
        Title:       "Echo",
        Description: message,
    }

    return dep.Responder.Send(dep.Interaction, rp.MessageOptions{
        Embeds:    []*dg.MessageEmbed{&embed},
        Ephemeral: true,
    })
}
```

### Best Practices

1. Always use `dep.Responder.Defer()` at the start of your handler for longer operations
2. Use the context for database operations
3. Handle errors appropriately and return them from the handler
4. Use embeds for structured responses
5. Consider using ephemeral messages for user-specific responses
6. Use the provided dependencies instead of creating new connections
7. Keep commands focused on a single responsibility

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License - see the LICENSE file for details. 
