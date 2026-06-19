# Auth Flow

How Cambium handles registration, login, token lifecycle, and protected routes.

---

## Token model

Cambium issues two tokens on every successful auth:

| Token | Lifetime | Storage | Transport |
|---|---|---|---|
| Access token | 15 minutes | Client (`localStorage`) | `Authorization: Bearer` header |
| Refresh token | 7 days | Server DB (hashed) + client cookie | `httpOnly` cookie |

The access token is a signed JWT (HS256). It carries `user_id` in the `sub` claim and is verified by Cambium on every protected request without a DB lookup — the signature is sufficient.

The refresh token is a random 32-byte hex string. Only a SHA-256 hash is stored in the DB. The raw token travels only in an `httpOnly` cookie — JavaScript cannot read it.

---

## Registration

```
POST /auth/register  {"email":"...", "password":"..."}

1. bcrypt.GenerateFromPassword(password, cost=12)
2. INSERT INTO cambium.users (email, password_hash)
3. IssueAccessToken(user_id)     → JWT, 15-min expiry
4. GenerateRefreshToken()        → 32 random bytes → hex string
5. HashToken(raw)                → SHA-256
6. INSERT INTO cambium.refresh_tokens (user_id, token_hash, expires_at)
7. Set-Cookie: refresh_token=<raw>; HttpOnly; SameSite=Strict
8. Return: {"access_token": "eyJ..."}
```

---

## Login

Same as registration but step 1 is:

```
bcrypt.CompareHashAndPassword(stored_hash, password)
```

Both registration and login return `401 Unauthorized` with the same generic error
message on failure — no difference between "email not found" and "wrong password"
to prevent user enumeration.

---

## Token refresh

```
POST /auth/refresh  (cookie: refresh_token=<raw>)

1. SHA-256(raw_token) → hash
2. SELECT * FROM refresh_tokens WHERE token_hash = hash
3. Check: revoked_at IS NULL  →  reject if revoked
4. Check: expires_at > NOW()  →  reject if expired
5. UPDATE refresh_tokens SET revoked_at = NOW() WHERE id = ...   ← revoke old
6. Issue new token pair (steps 3–8 from registration)
```

**Rotation on every use** — a refresh token can only be used once. This prevents
replay attacks: if an attacker steals the cookie and uses it first, the legitimate
user's next refresh fails, signalling the breach.

---

## JWT middleware

Every `/api/v1` route runs `RequireAuth` before its handler:

```
Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...

1. Parse header — reject if missing or not "Bearer ..."
2. jwt.ParseWithClaims(token, JWT_SECRET) → verify signature
3. Check: token.Valid && !expired
4. Extract sub claim → user_id
5. context.WithValue(r.Context(), userIDKey, user_id) → inject into request
6. call next handler
```

If any step fails, the request is rejected with `401 Unauthorized` before any
handler logic runs.

---

## Security properties

- **Passwords** — bcrypt with cost 12. ~250ms to hash intentionally; makes brute force slow.
- **Access tokens** — stateless, verified by signature alone. No DB lookup per request.
- **Refresh tokens** — stored only as SHA-256 hashes. DB breach doesn't expose raw tokens.
- **Provider keys** — AES-256-GCM encrypted with `CAMBIUM_ENCRYPTION_KEY`. Never returned to client.
- **user_id trust** — always from the verified JWT `sub` claim. Never from a request body parameter.
- **httpOnly cookie** — refresh token is inaccessible to JavaScript. XSS cannot steal it.

---

## Token expiry and re-authentication

When the access token expires (15 min), the client should:

1. Call `POST /auth/refresh` with the cookie — receive a new access token
2. If refresh also fails (expired or revoked) → redirect to login

The 15-minute access token lifetime is a deliberate trade-off: short enough to
limit damage from a stolen token, long enough for a normal session without
constant refreshes. Verdant should refresh proactively (e.g. at 12 minutes) to
avoid mid-action expiry.
