# Mnemosyne

Mnemosyne is an local MCP server that provides a simple interface for managing different projects memories. It allows you to create, read, update, and delete memories associated with various projects. The server is designed to be lightweight and easy to use, making it ideal for developers who need a quick way to manage project-related data.

IMPORTANT feature of Mnemosyne is its ability to handle multiple projects simultaneously, allowing users to switch between different project contexts without losing any data. Each project can have its own set of memories, which can be easily accessed and modified through the provided API.
Performance is optimized for speed and efficiency, ensuring that users can quickly retrieve and update memories as needed.

## Stack

### Backend

- Go

### Frontend

- Electron
- React
- TypeScript

### Database

- SQLite
- sqlite-vec


## Memory Management

For memory management, Mnemosyne uses a combination of Markdown + Json metadata + vector indexing. This allows for efficient storage and retrieval of memories, as well as the ability to perform complex queries on the stored data.

## Initial Setup

```
                    ┌───────────────┐
                    │ React + TS     │
                    │ Vite           │
                    │ Tailwind       │
                    │ React Flow     │
                    └───────┬───────┘
                            │
                       REST / SSE
                            │
                    ┌───────▼───────┐
                    │      Go       │
                    │ Memory API    │
                    └───────┬───────┘
                            │
             ┌──────────────┼──────────────┐
             ▼              ▼              ▼
        SQLite         pgvector       Markdown
```