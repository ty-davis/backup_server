# Backup Server

A simple web application for sharing files across users and groups, built in Go.

## Features

- User authentication with bcrypt password hashing
- Group-based access control with many-to-many relationships
- Secure file downloads
- Session management
- SQLite database
- Admin panel for managing files and users
- **Integrated TerraMap viewer for Terraria world files (.wld)**

## Setup

1. Install dependencies:
```bash
go mod download
```

2. Initialize database with sample data:
```bash
go run cmd/init/init.go
```

3. Run the server:
```bash
go run cmd/server/main.go
```

4. Access at http://localhost:8080

## Default Users

After initialization:
- Username: `admin` / Password: `admin` (Group: admins)
- Username: `user1` / Password: `password` (Group: users)
- Username: `user2` / Password: `password` (Groups: users, admins)

## Database Schema

- **users**: User accounts with credentials
- **groups**: Group definitions
- **user_groups**: Many-to-many relationship between users and groups
- **files**: File metadata with group-based access control

## Admin Panel

Users in the "admins" group can access the admin panel at `/admin/files`, `/admin/users`, and `/admin/groups` to:

**Manage Files:**
- Add, edit, and delete downloadable files
- Assign files to groups

**Manage Users:**
- Create and manage users
- Assign users to multiple groups
- Change user passwords
- Delete users

**Manage Groups:**
- Create new groups
- Rename existing groups
- Delete groups (if no files are assigned)
- View member and file counts per group

## Security

- Passwords hashed with bcrypt
- Session-based authentication with HttpOnly cookies
- Path validation prevents directory traversal
- Group-based authorization for file access

## TerraMap Integration

Terraria world files (`.wld`) automatically get a **"View Map"** button that opens an interactive map viewer. The viewer:
- Loads world files directly from the server (no manual upload needed)
- Respects group-based permissions
- Allows panning, zooming, and searching for blocks, ores, items, NPCs, etc.
- Opens in a new tab with full TerraMap functionality

Simply add a `.wld` file to your backup server, and users with access can view the map with one click!

