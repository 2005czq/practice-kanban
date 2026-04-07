# Practice Kanban

Practice Kanban is a static Codeforces virtual-practice dashboard designed for GitHub Pages.

The repository contains:

- a lightweight Go generator that fetches Codeforces data and produces static JSON
- a static frontend that reads `docs/data/dashboard.json`
- a GitHub Actions workflow that regenerates the site on schedule and deploys it to GitHub Pages

There is no always-on backend anymore. GitHub Actions is the only data-refresh backend.

## How It Works

1. GitHub Actions reads the same environment values you wanted before: `USERS`, `TIME`, `TITLE`
2. The Go generator fetches Codeforces API data with internal rate limiting of one request every two seconds
3. It writes:
   - `data/cache.json` for repository-visible cached output
   - `data/contest-metadata.json` for cached contest problem metadata
   - `docs/data/dashboard.json` for the static frontend
   - `docs/index.html` and `docs/assets/*` for GitHub Pages
4. The workflow commits refreshed data back to the repository and deploys `docs/` to GitHub Pages

## API Efficiency

The generator is optimized for GitHub Actions runtime:

- `contest.list` is fetched once
- `user.info?handles=...` is fetched once for all users, so current and max rating do not require per-user calls
- `user.status` is fetched once per user
- contest problem metadata is cached in `data/contest-metadata.json`

That means steady-state runs need only `n + 2` Codeforces API calls, where `n` is the number of users.

The only time extra calls are needed is when a newly included contest appears and its problem order is not yet present in the local metadata cache.

## Configuration

Required variables:

```env
USERS=Zihim,yangxuan,cjtyyds,Lx2024,hesto,Hone7
TIME=20260309
TITLE="Practice Kanban"
```

Optional variables:

```env
TZ=Asia/Shanghai
CACHE_FILE=data/cache.json
```

Notes:

- `TIME` uses `YYYYMMDD`
- `TIME` is interpreted at local midnight in `TZ`
- If `TZ` is not set in GitHub Actions, the workflow falls back to `Asia/Shanghai`

## Local Usage

1. Create a local env file:

	```bash
	cp .env.example .env
	```

2. Generate the site:

	```bash
	go run .
	```

3. Generated output:

	```bash
	docs/index.html
	docs/assets/app.js
	docs/assets/styles.css
	docs/data/dashboard.json
	data/cache.json
	data/contest-metadata.json
	```

4. Preview locally with any static file server if you want, for example:

	```bash
	python3 -m http.server 8080 -d docs
	```

	Then open `http://localhost:8080`.

## GitHub Pages Setup

1. Push this repository to GitHub.
2. In repository settings, enable GitHub Pages with GitHub Actions as the source.
3. Add repository secrets.

4. The workflow file is [pages.yml](./.github/workflows/pages.yml).

It will:

- run hourly
- regenerate static data
- commit updated `data/cache.json`, `data/contest-metadata.json`, and `docs/`
- deploy `docs/` to GitHub Pages

You can also run it manually with `workflow_dispatch`.

## Data Rules

The generator keeps the same contest-selection rules as before:

- only `Div. 2` and `Div. 1 + Div. 2`
- only contests whose first virtual participation starts on or after `TIME`
- exclude a contest if the user had any submission in that contest before that virtual start
- count wrong attempts before the first accepted submission

## Project Structure

```text
.
├── .env
├── .env.example
├── .github
│   └── workflows
│       └── pages.yml
├── .gitignore
├── README.md
├── data
│   ├── cache.json
│   └── contest-metadata.json
├── docs
│   ├── assets
│   │   ├── app.js
│   │   └── styles.css
│   ├── data
│   │   └── dashboard.json
│   └── index.html
├── go.mod
├── internal
│   ├── app
│   │   ├── config.go
│   │   ├── dotenv.go
│   │   └── generator.go
│   ├── codeforces
│   │   └── client.go
│   └── dashboard
│       ├── service.go
│       └── types.go
├── main.go
└── ui
    ├── embed.go
    └── web
        ├── app.js
        ├── index.html
        └── styles.css
```

## LICENSE

[MIT](./LICENSE)