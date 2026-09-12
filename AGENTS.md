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

## For AI agents

Use the @development folder to create a TODO and all the dev docs over their for this scope of developement.


## How things look like

Desktop app will have a simple interface with a siderbar for project selection and a main area for displaying and managing memories. Users can easily switch between projects, view existing memories, and add new ones. The interface is designed to be intuitive, allowing users to quickly navigate through their projects and access the information they need.

User can also configure the actual path of the memory storage, allowing for flexibility in how and where memories are stored. This can be particularly useful for users who want to integrate Mnemosyne with other tools or systems they are using.

GUI will provide a simple way to visualize the relationships between different memories, making it easier to understand how various pieces of information are connected. This can be especially helpful for complex projects with many interrelated components. Same graph view as Obsedian offers.

GUI will also include a search functionality, allowing users to quickly find specific memories based on keywords or metadata. This can save time and improve productivity, especially for users managing large amounts of data.

GUI will also have a simple way for integration with AI tools like MCP section where it will list down the command to connect it will diffrent AI tools like Claude code or Codex. 

Backend will provide a REST API for managing memories, allowing developers to integrate Mnemosyne with other applications or services. The API will support standard CRUD operations, making it easy to create, read, update, and delete memories programmatically.

IT will have all the necessary endpoints for managing projects and their associated memories, as well as endpoints for searching and retrieving specific memories based on various criteria.

All things will be saved in the formate it mention and also encrypted for security and privacy. Users can be assured that their data is protected and only accessible to authorized individuals.

It will have a backend and restore functionality for backup and recovery, allowing users to safeguard their memories and restore them in case of data loss or corruption. This can provide peace of mind for users who rely on Mnemosyne for managing critical project information.

It will also have a simple way to export and import memories, allowing users to easily transfer their data between different instances of Mnemosyne or share it with other users. This can be particularly useful for collaborative projects or when migrating to a new system.

It will also have a token counter for the AI agents to keep track of the number of tokens used in each project, helping users manage their usage and avoid exceeding any limits set by the AI tools they are using.

Also counter will tell the user how many tokens are getting saved in the memory for each project, allowing users to optimize their memory usage and ensure that they are making the most efficient use of their resources.


IMPORTANT: It will use https://github.com/colbymchenry/codegraph  for the graph view of the memories and also for the codebase visualization. It will provide a clear and interactive representation of the relationships between different pieces of information, making it easier for users to understand and navigate their project data.

This project comes under MIT, allowing for free use and distribution of the software. This will be comes under the binary of this app so that user dont have to install it separately. It will be used for the graph view of the memories and also for the codebase visualization. It will provide a clear and interactive representation of the relationships between different pieces of information, making it easier for users to understand and navigate their project data.


## For Git

Check the @development/Commit.md for the commit guidelines and rules for this project. It is important to follow these guidelines to maintain a clear and consistent history of changes, making it easier for team members to understand the purpose of each commit.