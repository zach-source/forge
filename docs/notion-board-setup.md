# Notion Board Setup for Forge

This guide walks you through setting up a Notion database for use with `forge board`.

## Quick Start

1. Create a new Notion database
2. Add the properties below
3. Create an integration and share the database
4. Configure forge with `forge board --config`

---

## Database Properties

Create a **Full Page Database** in Notion with these properties:

### Required Properties

| Property | Type | Configuration |
|----------|------|---------------|
| **Title** | Title | (default, rename to "Name" or "Feature") |
| **Status** | Select | Options: `Backlog`, `Ready`, `In Progress`, `Done` |
| **Type** | Select | Options: `Epic`, `Feature`, `Task` |

### Recommended Properties

| Property | Type | Configuration |
|----------|------|---------------|
| **Priority** | Select | Options: `P0`, `P1`, `P2`, `P3` |
| **Parent** | Relation | Self-relation to same database (for hierarchy) |
| **Bead ID** | Text | Auto-filled by forge sync |
| **Description** | Text | Feature details and acceptance criteria |
| **Assignee** | Person | Who's working on it |
| **Due Date** | Date | Target completion |
| **Labels** | Multi-select | Custom tags (e.g., `frontend`, `api`, `bug`) |

### Property Setup Details

#### Status (Select)
```
Backlog     → Gray    (not started, low priority)
Ready       → Blue    (ready to work on)
In Progress → Yellow  (actively being worked)
Done        → Green   (completed)
```

#### Type (Select)
```
Epic        → Purple  (large initiative, contains features)
Feature     → Blue    (deliverable unit of work)
Task        → Gray    (small piece of a feature)
```

#### Priority (Select)
```
P0          → Red     (critical, do immediately)
P1          → Orange  (high, do this sprint)
P2          → Yellow  (medium, do soon)
P3          → Gray    (low, backlog)
```

#### Parent (Relation)
- Create a self-relation to the same database
- Name it "Parent"
- This enables epic → feature → task hierarchy

---

## Database Views

Create these views for different workflows:

### 1. Kanban Board (Default)
- **View type:** Board
- **Group by:** Status
- **Sort:** Priority (ascending)
- **Filter:** None (show all)

### 2. Epic Overview
- **View type:** Table
- **Filter:** Type = Epic
- **Sort:** Priority, then Status
- **Show:** Title, Status, Priority, child count

### 3. Ready Queue
- **View type:** List
- **Filter:** Status = Ready
- **Sort:** Priority (ascending)
- **Show:** Title, Type, Priority, Parent

### 4. My Work
- **View type:** List
- **Filter:** Assignee = Me, Status != Done
- **Sort:** Priority, then Due Date
- **Show:** Title, Status, Priority, Due Date

### 5. Sprint Board
- **View type:** Board
- **Group by:** Status
- **Filter:** Due Date within next 2 weeks
- **Sort:** Priority

---

## Integration Setup

### Step 1: Create Integration

1. Go to [notion.so/my-integrations](https://www.notion.so/my-integrations)
2. Click **+ New integration**
3. Configure:
   - **Name:** `Forge Board Sync`
   - **Logo:** Optional
   - **Associated workspace:** Your workspace
4. Click **Submit**
5. Copy the **Internal Integration Token** (starts with `secret_`)

### Step 2: Share Database

1. Open your database in Notion
2. Click **...** (top right) → **Add connections**
3. Search for "Forge Board Sync"
4. Click to add the connection

### Step 3: Get Database ID

The database ID is in the URL:
```
https://www.notion.so/workspace/abc123def456...?v=...
                         └──────────────────┘
                         This is the database ID
```

Copy the 32-character hex string (may include dashes).

### Step 4: Configure Forge

```bash
# Set the API token
export NOTION_API_TOKEN=secret_xxxxxxxxxxxxx

# Add to your shell profile for persistence
echo 'export NOTION_API_TOKEN=secret_xxxxxxxxxxxxx' >> ~/.zshrc

# Configure the database
forge board --config
# Paste your database ID when prompted

# Verify setup
forge board --show
```

---

## Status Mapping

Forge maps statuses between Notion and beads:

| Notion Status | Bead Status | Sync Direction |
|---------------|-------------|----------------|
| Backlog | pending | Notion → Bead |
| Ready | pending | Notion → Bead |
| In Progress | active | Bidirectional |
| Done | completed | Bidirectional |

---

## Hierarchy Mapping

```
Notion                          Beads
──────────────────────────────────────
Epic: "Auth System"      →      bead-auth-system
  └─ Feature: "Login"    →      bead-login (parent: bead-auth-system)
       └─ Task: "UI"     →      bead-login-ui (parent: bead-login)
```

The Parent relation in Notion becomes bead dependencies.

---

## Example Database

Here's a sample starting structure:

```
┌─────────────────────────────────────────────────────────────┐
│ Feature Board                                               │
├─────────────────────────────────────────────────────────────┤
│ Backlog        │ Ready          │ In Progress   │ Done     │
├────────────────┼────────────────┼───────────────┼──────────┤
│ Epic: Billing  │ Feature: Login │ Task: UI      │ Setup CI │
│   P2           │   P1           │   P1          │          │
│                │                │               │          │
│ Feature: Logs  │ Task: API      │               │          │
│   P3           │   P1           │               │          │
└────────────────┴────────────────┴───────────────┴──────────┘
```

---

## Troubleshooting

### "NOTION_API_TOKEN not set"
```bash
export NOTION_API_TOKEN=secret_xxx
# Or add to ~/.zshrc / ~/.bashrc
```

### "Database not found" or 401 errors
- Verify the database is shared with your integration
- Check the database ID is correct (32 chars, from URL)
- Ensure the token is for the right workspace

### Sync not picking up items
- Check the Status property uses exact names: `Backlog`, `Ready`, `In Progress`, `Done`
- Verify Type property exists with `Epic`, `Feature`, `Task` options

### Missing properties in sync
- Bead ID property must be a Text type (not Number)
- Description should be Text or Rich Text

---

## Next Steps

After setup:

```bash
# Run initial sync
forge board --sync

# Start interactive session
forge board

# Monitor continuously
forge board --watch
```
