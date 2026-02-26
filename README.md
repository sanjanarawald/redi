# Redi – College Reddit-style Backend

Go backend for a Reddit-like app for college: students post (text and/or images), view a feed, and interact via likes, comments, replies, and saves.

## Stack

- **Go 1.21+**
- **Chi** router
- **Firebase Firestore** – database (users, posts, comments, likes, saves)
- **Firebase Storage** – post images (public URLs stored in Firestore)
- **JWT** auth, **bcrypt** passwords

Configuration is read from **environment variables** so secrets are not in code. See `.env.example` for the list.

## Environment variables

| Variable | Description | Default (local dev) |
|----------|-------------|----------------------|
| `REDI_PORT` | Server port | `8080` |
| `REDI_JWT_SECRET` | JWT signing secret (set a long random string in production) | `test-jwt-secret-change-in-production` |
| `REDI_FIREBASE_PROJECT_ID` | Firebase project ID | *(required)* |
| `REDI_FIREBASE_STORAGE_BUCKET` | Storage bucket, e.g. `your-project.firebasestorage.app` | *(required)* |
| `REDI_FIREBASE_CREDENTIALS_PATH` | Path to service account JSON key file | `serviceAccountKey.json` |

Copy `.env.example` to `.env`, fill in your values, and load them before running (e.g. `export $(cat .env | xargs)` on Unix, or use a tool like `direnv`). **Do not commit `.env` or `serviceAccountKey.json`.**

## Firebase setup

1. Create a Firebase project and enable **Firestore** and **Storage**.
2. In Firebase Console → Project settings → Service accounts, generate a new private key. Save as `serviceAccountKey.json` in the project root (or another path and set `REDI_FIREBASE_CREDENTIALS_PATH`).
3. Set the env vars above: `REDI_FIREBASE_PROJECT_ID`, `REDI_FIREBASE_STORAGE_BUCKET`, and optionally `REDI_FIREBASE_CREDENTIALS_PATH`.

4. **Firestore indexes** (create when the app first uses them, or in Firebase Console → Firestore → Indexes):
   - Collection `comments`: fields `post_id` (Ascending), `created_at` (Ascending)
   - Collection `saves`: fields `user_id` (Ascending), `created_at` (Descending)

5. **Storage**: ensure your Storage bucket allows uploads (Blaze plan if required). Post images are stored under `posts/<id>.<ext>` and served via public URLs.

## Build and run

```bash
cd redi
go mod tidy
go build -o redi.exe .
.\redi.exe
```

Server listens on `:8080` by default (set `REDI_PORT` to change).

## Deploy to GitHub

1. **Confirm secrets are not committed**  
   Ensure `.gitignore` is present and includes `.env` and `serviceAccountKey.json`. Never commit your Firebase key file or `.env`.

2. **Initialize Git (if this folder is not yet a repo)**  
   ```bash
   cd redi
   git init
   ```

3. **Create a new repository on GitHub**  
   - Go to [github.com/new](https://github.com/new).  
   - Name it (e.g. `redi`).  
   - Do **not** add a README, .gitignore, or license (you already have them).  
   - Click **Create repository**.

4. **Add and push your code**  
   Replace `YOUR_USERNAME` and `YOUR_REPO` with your GitHub username and repo name.  
   ```bash
   git add .
   git commit -m "Initial commit: Redi backend and web UI"
   git branch -M main
   git remote add origin https://github.com/YOUR_USERNAME/YOUR_REPO.git
   git push -u origin main
   ```

5. **If you use SSH instead of HTTPS**  
   ```bash
   git remote add origin git@github.com:YOUR_USERNAME/YOUR_REPO.git
   git push -u origin main
   ```

6. **Optional – add a README badge or repo description**  
   In GitHub: **Settings → General** or edit the README to add a short description.

Anyone cloning the repo will need to add their own `serviceAccountKey.json` and set the environment variables (see `.env.example`).

## Test UI

A simple web UI is served at the root so you can try the app in the browser:

1. Start the backend: `.\redi.exe`
2. Open **http://localhost:8080** in your browser.
3. Register or log in, then use **Feed**, **New post**, **Saved**, and open any post to comment, like, reply, and save.

No separate frontend build or server is required.

## API

Base URL: `http://localhost:8080/api`

### Auth (no token)

- **POST** `/register` – body: `{"email","username","password"}` → `{user, token}`
- **POST** `/login` – body: `{"email","password"}` → `{user, token}`

All other endpoints need header: `Authorization: Bearer <token>`.

### Me

- **GET** `/me` – current user
- **GET** `/me/saved` – current user’s saved posts

### Posts

- **POST** `/posts` – create post. JSON: `{"content"}` or multipart: `content`, `image` (file). Image is uploaded to Firebase Storage; post in Firestore has `content` (text) and `image_url` (link to image).
- **GET** `/posts` – feed, query: `?limit=20&offset=0`
- **GET** `/posts/{id}` – one post
- **PATCH** `/posts/{id}` – update own post (`{"content"}`)
- **DELETE** `/posts/{id}` – delete own post

### Interactions

- **POST** `/posts/{id}/like` – like post  
- **DELETE** `/posts/{id}/like` – unlike  
- **POST** `/posts/{id}/save` – save post  
- **DELETE** `/posts/{id}/save` – unsave  

### Comments

- **GET** `/posts/{id}/comments` – list (threaded; replies in `replies`)
- **POST** `/posts/{id}/comments` – add comment; body: `{"content"}` or `{"content","parent_id"}` for reply
- **POST** `/comments/{id}/reply` – body: `{"content"}`
- **POST** `/comments/{id}/like` – like comment  
- **DELETE** `/comments/{id}/like` – unlike  

## Post shape in Firestore / API

Each post is stored in Firestore with:

- `content` – text
- `image_url` – link to image in Firebase Storage (optional)

API responses add `author_username`, `like_count`, `comment_count`, `liked_by_me`, `saved_by_me`.

Images are uploaded to Firebase Storage under `posts/<uuid>.<ext>` and the public URL is saved in the post document.
