# Token Package Documentation

## Overview

The `token` package provides a centralized token management system for temporary operations such as password resets, user registrations, project invitations, and scheme access. It implements a unified approach to handle time-limited tokens across different sections of the application.

**Package Location:** `internal/core/token`

## Architecture

The token system is built on a three-dimensional classification model:
- **Section**: Defines the domain/scope of the token (profile, project, scheme, agent)
- **Action**: Specifies the operation type (reset, register, add, delete, etc.)
- **Status**: Tracks the token lifecycle state (sent, used, done, deleted, archived)

### Handler Structure

```go
type Handler struct {
    tokenpb.UnimplementedTokenHandlersServer
    DB     *postgres.Connect
    Worker worker.Client
}
```

The handler requires:
- **DB**: PostgreSQL database connection for token storage
- **Worker**: Worker client for asynchronous operations (e.g., email notifications)

## Database Schema

The `token` table structure:

```sql
CREATE TABLE "token" (
    "id" uuid DEFAULT gen_random_uuid() PRIMARY KEY,
    "section" smallint NOT NULL,
    "action" smallint NOT NULL,
    "status" smallint NOT NULL,
    "profile_id" uuid DEFAULT NULL REFERENCES "profile"("id") ON DELETE CASCADE,
    "project_id" uuid DEFAULT NULL REFERENCES "project"("id") ON DELETE CASCADE,
    "scheme_id" uuid DEFAULT NULL REFERENCES "scheme"("id") ON DELETE CASCADE,
    "data" jsonb NOT NULL DEFAULT '{}'::jsonb,
    "expired_at" timestamp DEFAULT NULL,
    "updated_at" timestamp DEFAULT NULL,
    "created_at" timestamp DEFAULT NOW()
);
```

**Indexes:**
- Primary key on `id`
- Indexes on `section`, `action`, `status`
- Foreign key indexes on `profile_id`, `project_id`, `scheme_id`

## Token Classification

### Section (Domain)

Defines the application area where the token is used:

| Value | Name | Description |
|-------|------|-------------|
| 0 | `section_unspecified` | Default/unspecified |
| 1 | `profile` | Profile-related operations |
| 2 | `project` | Project-related operations |
| 3 | `scheme` | Scheme-related operations |
| 4 | `agent` | Agent-related operations |
| 5 | `rest` | Other operations |

### Action (Operation Type)

Specifies the type of operation the token authorizes:

| Value | Name | Description |
|-------|------|-------------|
| 0 | `action_unspecified` | Default/unspecified |
| 1 | `reset` | Password reset |
| 2 | `register` | User registration |
| 3 | `add` | Add/create operation |
| 4 | `delete` | Delete operation |
| 5 | `disable` | Disable operation |
| 6 | `enable` | Enable operation |
| 7 | `request` | Request/invitation |
| 8 | `access` | Access grant |

### Status (Lifecycle State)

Tracks the current state of the token:

| Value | Name | Description |
|-------|------|-------------|
| 0 | `status_unspecified` | Default/unspecified |
| 1 | `done` | Operation completed |
| 2 | `sent` | Token sent (active) |
| 3 | `used` | Token used |
| 4 | `deleted` | Token deleted |
| 5 | `archived` | Token archived |

## Core Components

### 1. Token Retrieval

#### `Token(ctx, in)`

Retrieves complete token information by token ID. Automatically determines the owner based on the section and includes metadata parsing.

**Features:**
- Owner ID calculation based on section type
- Metadata deserialization (Profile, Scheme, Agent)
- Secret masking for non-admin users
- Timestamp conversion
- Token expiration validation (checks `expired_at` if set)

**Example:**
```go
tokenData, err := tokenHandler.Token(ctx, &tokenmessage.Token_Request{
    IsAdmin: false,
    Token:   tokenID,
})
```

### 2. Active Token Search

#### `FindActiveTokenByProfileAndAction(ctx, profileID, action)`

Searches for an active token by profile ID and action. A token is considered active if:
- Status is `sent`
- Created within the last 24 hours OR `expired_at` is in the future (if set)
- Matches the specified profile and action

**Returns:**
- Token ID (string) if found
- Empty string if not found
- Error on database failure

**Note:** If `expired_at` is set, it takes precedence over the 24-hour default. If `expired_at` is NULL, the 24-hour window from creation time is used.

**Example:**
```go
tokenID, err := handler.FindActiveTokenByProfileAndAction(
    ctx, 
    profileID, 
    tokenenum.Action_reset,
)
```

### 3. Get or Create Token

#### `GetOrCreateProfileToken(ctx, profileID, action, createToken)`

High-level function that checks for an existing active token and creates a new one if none exists. Prevents duplicate token creation.

**Parameters:**
- `profileID`: Profile identifier
- `action`: Token action type
- `createToken`: Callback function to create a new token

**Returns:**
- `tokenID`: Token identifier
- `isNew`: Boolean indicating if token was newly created
- `error`: Error if operation fails

**Example:**
```go
tokenID, isNew, err := handler.GetOrCreateProfileToken(
    ctx,
    profileID,
    tokenenum.Action_reset,
    func(ctx context.Context, profileID string) (string, error) {
        resp, err := handler.AddTokenProfileReset(ctx, &tokenmessage.AddTokenProfileReset_Request{
            ProfileId: profileID,
        })
        if err != nil {
            return "", err
        }
        return resp.GetToken(), nil
    },
)
```

### 4. Profile Data Retrieval

#### `GetProfileDataByEmail(ctx, email)`

Retrieves profile information by email address. Returns a structure indicating whether the profile exists.

**Returns:**
- `ProfileDataByEmail` struct with:
  - `ID`: Profile UUID
  - `Name`: Profile name
  - `Surname`: Profile surname
  - `Exists`: Boolean indicating profile existence

**Example:**
```go
profileData, err := handler.GetProfileDataByEmail(ctx, "user@example.com")
if err != nil {
    return err
}
if profileData.Exists {
    // Profile found
}
```

### 5. Token Expiration Validation

The token system supports flexible expiration validation:

- **Custom expiration**: If `expired_at` is set when creating a token, it takes precedence over the default 24-hour window
- **Default expiration**: If `expired_at` is NULL, tokens are valid for 24 hours from creation time
- **Automatic validation**: Both `Token` and `FindActiveTokenByProfileAndAction` methods automatically validate expiration

**Implementation:**
- `Token` method checks `expired_at` before returning token data and returns an error if expired
- `FindActiveTokenByProfileAndAction` includes `expired_at` check in SQL query for efficient filtering

**Example:**
```go
// Token with custom expiration (48 hours)
resp, err := handler.AddTokenProfileReset(ctx, &tokenmessage.AddTokenProfileReset_Request{
    ProfileId: profileID,
    ExpiredAt: timestamppb.New(time.Now().Add(48 * time.Hour)),
})

// Token lookup automatically respects expiration
tokenID, err := handler.FindActiveTokenByProfileAndAction(ctx, profileID, tokenenum.Action_reset)
```

### 6. Rate Limiting

The package provides built-in rate limiting functionality to prevent token abuse:

#### `CountRecentTokensByProfileAndAction(ctx, profileID, action, section)`

Counts tokens created within the last 24 hours for a specific profile, action, and section combination.

**Use case**: Determine how many tokens have been created recently for rate limiting decisions.

#### `CheckRateLimit(ctx, profileID, action, section, maxTokensPer24h)`

Checks if token creation rate limit is exceeded. Returns `true` if the limit is reached.

**Parameters:**
- `maxTokensPer24h`: Maximum number of tokens allowed per 24 hours

**Example:**
```go
// Check rate limit before creating token
exceeded, err := handler.CheckRateLimit(
    ctx,
    profileID,
    tokenenum.Action_reset,
    tokenenum.Section_profile,
    5, // max 5 tokens per 24 hours
)
if err != nil {
    return err
}
if exceeded {
    return errors.New("rate limit exceeded: too many token requests")
}

// Proceed with token creation
tokenID, err := handler.AddTokenProfileReset(ctx, &tokenmessage.AddTokenProfileReset_Request{
    ProfileId: profileID,
})
```

### 7. Token Usage Metrics

#### `GetTokenMetrics(ctx, section, action)`

Retrieves basic token usage statistics by section and action.

**Returns:**
- `TokenMetrics` struct containing:
  - `Section`: Token section type
  - `Action`: Token action type
  - `Created`: Count of created tokens (status = `sent`)
  - `Used`: Count of used tokens (status = `used`)

**Use case**: Monitor token usage patterns, track success rates, and identify potential issues.

**Example:**
```go
metrics, err := handler.GetTokenMetrics(
    ctx,
    tokenenum.Section_profile,
    tokenenum.Action_reset,
)
if err != nil {
    return err
}

fmt.Printf("Created: %d, Used: %d\n", metrics.Created, metrics.Used)
// Calculate success rate
if metrics.Created > 0 {
    successRate := float64(metrics.Used) / float64(metrics.Created) * 100
    fmt.Printf("Success rate: %.2f%%\n", successRate)
}
```

## API Methods

### Profile Tokens

#### `ProfileTokens(ctx, in)`

Lists profile tokens with pagination. **Admin only.**

**Request:**
- `IsAdmin`: Must be true
- `Action`: Filter by action (optional)
- `Status`: Filter by status (optional)
- `Limit`, `Offset`, `SortBy`: Pagination parameters

**Response:**
- `Total`: Total count of matching tokens
- `Tokens`: Array of token responses

#### `AddTokenProfileReset(ctx, in)`

Creates a password reset token for a profile.

**Request:**
- `ProfileId`: Profile UUID (required)
- `ExpiredAt`: Optional expiration timestamp

**Response:**
- `Token`: Generated token UUID

**Workflow:**
1. Validates profile existence
2. Creates token with:
   - Section: `profile`
   - Action: `reset`
   - Status: `sent`
3. Returns token ID

#### `AddTokenProfileRegistration(ctx, in)`

Creates a registration token for new profile creation.

**Request:**
- `Data`: Profile metadata (name, email, etc.)
- `ExpiredAt`: Optional expiration timestamp

**Response:**
- `Token`: Generated token UUID

#### `AddTokenProfileDelete(ctx, in)`

Creates a profile deletion confirmation token.

**Request:**
- `ProfileId`: Profile UUID (required)
- `ExpiredAt`: Optional expiration timestamp

**Response:**
- `Token`: Generated token UUID

#### `UpdateProfileToken(ctx, in)`

Updates a profile token status and optionally adds profile_id for registration tokens.

**Request:**
- `Token`: Token UUID
- `Status`: New status
- `ProfileId`: Required when updating registration token to `done`
- `IsAdmin`: Admin flag

**Special Logic:**
- Registration tokens (`Action_register`) transitioning to `done` require `ProfileId`

### Project Tokens

#### `ProjectTokens(ctx, in)`

Lists project tokens with pagination. Filters by project owner.

**Request:**
- `OwnerId`: Project owner UUID
- `ProjectId`: Project UUID
- `Action`: Filter by action (optional)
- `Status`: Filter by status (optional)
- `Limit`, `Offset`, `SortBy`: Pagination parameters

#### `AddTokenProjectMember(ctx, in)`

Creates a token for inviting a member to a project.

**Request:**
- `OwnerId`: Project owner UUID
- `ProjectId`: Project UUID
- `Profile`: Either `ProfileId` or `CreateNewProfile` data
- `ExpiredAt`: Optional expiration timestamp

**Response:**
- `Token`: Generated token UUID

**Workflow:**
1. Validates project existence and ownership
2. If profile exists, links to token
3. If new profile, stores registration data in token metadata
4. Creates token with:
   - Section: `project`
   - Action: `request`
   - Status: `sent`

#### `UpdateProjectToken(ctx, in)`

Updates a project token status.

**Request:**
- `Token`: Token UUID
- `Status`: New status
- `ProfileId`: Required when updating request token to `done` without existing profile_id
- `IsAdmin`: Admin flag

### Scheme Tokens

#### `SchemeTokens(ctx, in)`

Lists scheme tokens with pagination. Filters by scheme owner.

**Request:**
- `OwnerId`: Project owner UUID
- `SchemeId`: Scheme UUID
- `Action`: Filter by action (optional)
- `Status`: Filter by status (optional)
- `Limit`, `Offset`, `SortBy`: Pagination parameters

#### `AddTokenSchemeAdd(ctx, in)`

Creates a token for adding a scheme to a project.

**Request:**
- `OwnerId`: Project owner UUID
- `ProjectId`: Project UUID
- `Data`: Either `Email` or `ProfileId`
- `ExpiredAt`: Optional expiration timestamp

**Response:**
- `Token`: Generated token UUID

#### `AddTokenSchemeAccess(ctx, in)`

Creates a token for one-time web access to a scheme.

**Request:**
- `OwnerId`: Project owner UUID
- `SchemeId`: Scheme UUID
- `Data`: Either `Email` or `ProfileId` (optional)
- `ExpiredAt`: Optional expiration timestamp

**Response:**
- `Token`: Generated token UUID

#### `UpdateSchemeToken(ctx, in)`

Updates a scheme token status.

**Request:**
- `Token`: Token UUID
- `Status`: New status
- `SchemeId`: Required when updating add token to `done`
- `ProfileId`: Optional when updating add token to `done`
- `IsAdmin`: Admin flag

### Token Management

#### `DeleteToken(ctx, in)`

Marks a token as deleted (soft delete).

**Request:**
- `Token`: Token UUID
- `OwnerId`: Required for non-admin users
- `IsAdmin`: Admin flag

**Behavior:**
- Sets status to `deleted`
- Validates ownership for non-admin users

#### `ArchivedToken(ctx, in)`

Archives a token.

**Request:**
- `Token`: Token UUID
- `OwnerId`: Required for non-admin users
- `IsAdmin`: Admin flag

**Behavior:**
- Sets status to `archived`
- Validates ownership for non-admin users

## Utility Functions

### `buildInsertQuery(fields, args)`

Constructs a parameterized INSERT query for token creation.

**Parameters:**
- `fields`: Array of field names
- `args`: Array of field values

**Returns:**
- SQL query string with RETURNING clause
- Arguments array

### `handleSQLError(err)`

Converts PostgreSQL errors to gRPC errors with human-readable messages.

**Error Mapping:**
- `token_profile_id_fkey` → `MsgProfileNotFound`
- `token_project_id_fkey` → `MsgProjectNotFound`
- `token_scheme_id_fkey` → `MsgSchemeNotFound`
- Other errors → `MsgFailedToUpdate`

### `scanTokens(rows, limit, isAdmin)`

Scans database rows into token response objects.

**Features:**
- Handles nullable fields (profile_id, scheme_id)
- Converts timestamps
- Masks secrets for non-admin users

### `ValidateTokenStatusAndAction(tokenStatus, action, expectedAction, expectedStatus)`

Validates token status and action combination. Returns error if validation fails.

**Parameters:**
- `tokenStatus`: Current token status
- `action`: Current token action
- `expectedAction`: Expected action for validation
- `expectedStatus`: Expected status for validation

**Returns:**
- Error if validation fails, nil otherwise

### `ValidateTokenForUpdate(isAdmin, currentStatus, newStatus)`

Validates if token can be updated with new status. Returns error if validation fails.

**Parameters:**
- `isAdmin`: Whether the requester is an admin
- `currentStatus`: Current token status
- `newStatus`: New status to set

**Returns:**
- Error if validation fails, nil otherwise

**Validation Rules:**
- Non-admin users cannot set `status_unspecified`, `deleted`, or `archived` status
- Non-admin users cannot update tokens with `done` status

### `CountRecentTokensByProfileAndAction(ctx, profileID, action, section)`

Counts tokens created within the last 24 hours for rate limiting purposes.

**Parameters:**
- `ctx`: Context
- `profileID`: Profile identifier
- `action`: Token action type
- `section`: Token section type

**Returns:**
- Count of recent tokens and error

### `CheckRateLimit(ctx, profileID, action, section, maxTokensPer24h)`

Checks if token creation rate limit is exceeded.

**Parameters:**
- `ctx`: Context
- `profileID`: Profile identifier
- `action`: Token action type
- `section`: Token section type
- `maxTokensPer24h`: Maximum number of tokens allowed per 24 hours

**Returns:**
- Boolean indicating if rate limit is exceeded and error

### `GetTokenMetrics(ctx, section, action)`

Retrieves basic token usage metrics by section and action.

**Parameters:**
- `ctx`: Context
- `section`: Token section type
- `action`: Token action type

**Returns:**
- `TokenMetrics` struct with:
  - `Section`: Token section
  - `Action`: Token action
  - `Created`: Count of created tokens (status = sent)
  - `Used`: Count of used tokens (status = used)

## Usage Examples

### Password Reset Flow

```go
// Step 1: Request password reset
tokenHandler := token.Handler{DB: db, Worker: worker}
tokenID, isNew, err := tokenHandler.GetOrCreateProfileToken(
    ctx,
    profileID,
    tokenenum.Action_reset,
    func(ctx context.Context, profileID string) (string, error) {
        resp, err := tokenHandler.AddTokenProfileReset(ctx, &tokenmessage.AddTokenProfileReset_Request{
            ProfileId: profileID,
        })
        if err != nil {
            return "", err
        }
        return resp.GetToken(), nil
    },
)
if err != nil {
    return err
}

// Send email with token link
notification.SendMail(ctx, &notificationpb.SendMail_Request{
    Email:    email,
    Subject:  "reset password confirmation",
    Template: notificationpb.MailTemplate_password_reset,
    Data: map[string]string{
        "Link":      fmt.Sprintf("%s/auth/password_reset/%s", appDSN, tokenID),
        "FirstSend": strconv.FormatBool(isNew),
    },
})

// Step 2: Verify token
tokenData, err := tokenHandler.Token(ctx, &tokenmessage.Token_Request{
    IsAdmin: false,
    Token:   tokenID,
})
if err != nil {
    return err
}

// Validate token using common validation function
if err := token.ValidateTokenStatusAndAction(
    tokenData.GetStatus(),
    tokenData.GetAction(),
    tokenenum.Action_reset,
    tokenenum.Status_sent,
); err != nil {
    return err
}

// Step 3: Update password and mark token as used
tx, _ := db.Conn.BeginTx(ctx, nil)
defer tx.Rollback()

// Update password
tx.ExecContext(ctx, `UPDATE "profile" SET "password" = $1 WHERE "id" = $2`, 
    newPasswordHash, profileID)

// Mark token as used
tx.ExecContext(ctx, `UPDATE "token" SET "status" = $1 WHERE "id" = $2`,
    tokenenum.Status_used, tokenID)

tx.Commit()
```

### Project Invitation Flow

```go
// Create invitation token
tokenHandler := token.Handler{DB: db, Worker: worker}

// Check if profile exists
profileData, err := tokenHandler.GetProfileDataByEmail(ctx, email)
if err != nil {
    return err
}

var request *tokenmessage.AddTokenProjectMember_Request
if profileData.Exists {
    // Existing user invitation
    request = &tokenmessage.AddTokenProjectMember_Request{
        OwnerId:   ownerID,
        ProjectId: projectID,
        Profile: &tokenmessage.AddTokenProjectMember_Request_ProfileId{
            ProfileId: profileData.ID,
        },
    }
} else {
    // New user registration with project link
    request = &tokenmessage.AddTokenProjectMember_Request{
        OwnerId:   ownerID,
        ProjectId: projectID,
        Profile: &tokenmessage.AddTokenProjectMember_Request_CreateNewProfile{
            CreateNewProfile: &tokenmessage.MetaDataProfile{
                Email: email,
            },
        },
    }
}

resp, err := tokenHandler.AddTokenProjectMember(ctx, request)
if err != nil {
    return err
}

// Send invitation email
notification.SendMail(ctx, &notificationpb.SendMail_Request{
    Email:    email,
    Subject:  "project invitation",
    Template: notificationpb.MailTemplate_project_invitation,
    Data: map[string]string{
        "Link": fmt.Sprintf("%s/project/join/%s", appDSN, resp.GetToken()),
    },
})
```

## Security Features

### 1. Token Expiration

Tokens support flexible expiration validation:
- **Custom expiration**: If `expired_at` is set when creating a token, it takes precedence over the default 24-hour window
- **Default expiration**: If `expired_at` is NULL, tokens are valid for 24 hours from creation time
- **Automatic validation**: Both `Token` and `FindActiveTokenByProfileAndAction` methods automatically validate expiration

The `FindActiveTokenByProfileAndAction` function enforces expiration by checking both the 24-hour window and `expired_at` field in the SQL query. The `Token` method validates expiration before returning token data and returns an error if the token has expired.

### 2. Secret Masking

For non-admin users, sensitive fields are masked using the `ghoster.Secrets` function:
- Token ID
- Owner ID
- Profile ID
- Project ID
- Scheme ID

### 3. Ownership Validation

For project and scheme tokens, ownership is validated:
- Project tokens: Validated against project owner
- Scheme tokens: Validated through project ownership chain

### 4. Status Validation

Tokens can only be used in specific states:
- Password reset: Must be `sent` status
- Registration: Transitions from `sent` to `done`
- Deletion: Must be `sent` status

### 5. Duplicate Prevention

The `GetOrCreateProfileToken` function prevents creating duplicate active tokens by checking for existing tokens within the 24-hour window or before `expired_at` (if set).

### 6. Rate Limiting

The package provides built-in rate limiting to prevent token abuse. Use `CheckRateLimit` before creating tokens to enforce limits per profile/action combination. This helps prevent:
- Brute force attacks
- Token spam
- Resource exhaustion

### 7. Common Validation Functions

Reusable validation functions (`ValidateTokenStatusAndAction` and `ValidateTokenForUpdate`) ensure consistent validation logic across all token operations, reducing code duplication and potential security vulnerabilities.

## Best Practices

### 1. Always Use Helper Functions

Prefer `GetOrCreateProfileToken` over direct token creation to prevent duplicates:

```go
// ✅ Good
tokenID, isNew, err := handler.GetOrCreateProfileToken(...)

// ❌ Bad
tokenID, err := handler.AddTokenProfileReset(...)
```

### 2. Validate Token Before Use

Always validate token status and action before performing operations:

```go
tokenData, err := handler.Token(ctx, &tokenmessage.Token_Request{
    IsAdmin: false,
    Token:   tokenID,
})
if err != nil {
    return err
}

if tokenData.GetAction() != expectedAction || 
   tokenData.GetStatus() != tokenenum.Status_sent {
    return errors.New("invalid token")
}
```

### 3. Update Token Status in Transactions

When using a token, update its status within the same transaction:

```go
tx, _ := db.Conn.BeginTx(ctx, nil)
defer tx.Rollback()

// Perform operation
tx.ExecContext(ctx, `UPDATE "profile" SET ...`)

// Mark token as used
tx.ExecContext(ctx, `UPDATE "token" SET "status" = $1 WHERE "id" = $2`,
    tokenenum.Status_used, tokenID)

tx.Commit()
```

### 4. Handle Errors Gracefully

Use the error handling utilities for consistent error messages:

```go
if err != nil {
    return handleSQLError(err) // Converts DB errors to gRPC errors
}
```

### 5. Check Profile Existence

Before creating profile-related tokens, verify profile existence:

```go
profileData, err := handler.GetProfileDataByEmail(ctx, email)
if err != nil {
    return err
}
if !profileData.Exists {
    // Handle new profile case
}
```

### 6. Use Rate Limiting

Before creating tokens, check rate limits:

```go
// Check rate limit (max 5 tokens per 24 hours)
exceeded, err := handler.CheckRateLimit(
    ctx,
    profileID,
    tokenenum.Action_reset,
    tokenenum.Section_profile,
    5, // maxTokensPer24h
)
if err != nil {
    return err
}
if exceeded {
    return errors.New("rate limit exceeded: too many token requests")
}
```

### 7. Use Common Validation Functions

Use shared validation functions instead of manual checks:

```go
// ✅ Good: Use common validation
if err := token.ValidateTokenStatusAndAction(
    tokenData.GetStatus(),
    tokenData.GetAction(),
    expectedAction,
    expectedStatus,
); err != nil {
    return err
}

// ❌ Bad: Manual validation
if tokenData.GetAction() != expectedAction || 
   tokenData.GetStatus() != expectedStatus {
    return errors.New("invalid token")
}
```

### 8. Get Token Metrics

Retrieve basic usage statistics:

```go
metrics, err := handler.GetTokenMetrics(
    ctx,
    tokenenum.Section_profile,
    tokenenum.Action_reset,
)
if err != nil {
    return err
}
// metrics.Created - count of created tokens
// metrics.Used - count of used tokens
```

## Error Handling

The package uses consistent error handling through the `trace` package:

- **InvalidArgument**: Invalid input parameters
- **NotFound**: Token, profile, project, or scheme not found
- **PermissionDenied**: Insufficient permissions
- **FailedToAdd/Update/Delete**: Database operation failures

All errors are wrapped with context using `trace.Error()` for better debugging.

## Dependencies

- `github.com/werbot/werbot/pkg/storage/postgres`: Database connection
- `github.com/werbot/werbot/pkg/worker`: Worker client for async operations
- `github.com/werbot/werbot/internal/trace`: Error tracing and logging
- `google.golang.org/grpc`: gRPC status codes
- `google.golang.org/protobuf`: Protocol buffer encoding

## Related Packages

- **Profile Package** (`internal/core/profile`): Uses tokens for password reset and profile deletion
- **Project Package** (`internal/core/project`): Uses tokens for member invitations
- **Scheme Package** (`internal/core/scheme`): Uses tokens for scheme access

