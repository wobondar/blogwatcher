# BlogWatcher

A Go CLI tool to track blog articles, detect new posts, and manage read/unread status. Supports both RSS/Atom feeds and HTML scraping as fallback.

## Features

-   **Dual Source Support** - Tries RSS feeds first, falls back to HTML scraping
-   **Automatic Feed Discovery** - Detects RSS/Atom URLs from blog homepages
-   **Read/Unread Management** - Track which articles you've read
-   **Category Support** - View and filter articles by RSS/Atom categories
-   **Blog Topics** - Organize blogs by topics, filter across commands
-   **Blog Filtering** - View articles from specific blogs
-   **JSON Output** - Machine-readable output for scripting
-   **Status Dashboard** - Overview of blogs, articles, and topics at a glance
-   **Cleanup** - Remove old articles to keep the database lean
-   **Duplicate Prevention** - Never tracks the same article twice
-   **Colored CLI Output** - User-friendly terminal interface

## Installation

```bash
# Homebrew (Linux)
brew install Hyaxia/tap/blogwatcher

# Install the CLI
go install github.com/Hyaxia/blogwatcher/cmd/blogwatcher@latest

# Or build locally
go build ./cmd/blogwatcher
```

Windows and Linux binaries are also available on the GitHub Releases page.

## Usage

### Adding Blogs

```bash
# Add a blog (auto-discovers RSS feed)
blogwatcher add "My Favorite Blog" https://example.com/blog

# Add with explicit feed URL
blogwatcher add "Tech Blog" https://techblog.com --feed-url https://techblog.com/rss.xml

# Add with topics
blogwatcher add "Security Blog" https://secblog.com --topics "security,infosec"

# Add with HTML scraping selector (for blogs without feeds)
blogwatcher add "No-RSS Blog" https://norss.com --scrape-selector "article h2 a"
```

### Managing Blogs

```bash
# List all tracked blogs
blogwatcher blogs

# Filter blogs by topics
blogwatcher blogs --topics "security,go"

# Remove a blog (and all its articles)
blogwatcher remove "My Favorite Blog"

# Remove without confirmation
blogwatcher remove "My Favorite Blog" -y
```

### Scanning for New Articles

```bash
# Scan all blogs for new articles
blogwatcher scan

# Scan a specific blog
blogwatcher scan "Tech Blog"

# Scan only blogs with specific topics
blogwatcher scan --topics "security"
```

### Viewing Articles

```bash
# List unread articles
blogwatcher articles

# List all articles (including read)
blogwatcher articles --all

# List articles from a specific blog
blogwatcher articles --blog "Tech Blog"

# Filter by category (case-insensitive)
blogwatcher articles --category "AI"

# Filter by blog topics
blogwatcher articles --topics "security,go"

# Combine filters
blogwatcher articles --blog "Tech Blog" --category "Security"

# Output as JSON
blogwatcher articles --json
```

### Managing Read Status

```bash
# Mark articles as read (use article IDs from articles list)
blogwatcher read 42
blogwatcher read 1 2 3

# Mark articles as unread
blogwatcher unread 42
blogwatcher unread 5 6

# Mark all unread articles as read
blogwatcher read-all

# Mark all unread articles as read for a blog (skip prompt)
blogwatcher read-all --blog "Tech Blog" --yes

# Mark all unread articles as read for topics
blogwatcher read-all --topics "security" --yes
```

### Status

```bash
# Show database summary
blogwatcher status

# Output as JSON (useful for scripting)
blogwatcher status --json
```

### Cleanup

```bash
# Remove articles older than 365 days (default)
blogwatcher cleanup

# Remove articles older than 90 days
blogwatcher cleanup --days 90

# Skip confirmation
blogwatcher cleanup --days 90 --yes
```

### JSON Output

Most commands support `--json` for machine-readable output:

```bash
blogwatcher articles --json
blogwatcher blogs --json
blogwatcher status --json
blogwatcher cleanup --json
```

## How It Works

### Scanning Process

1. For each tracked blog, BlogWatcher first attempts to parse the RSS/Atom feed
2. If no feed URL is configured, it tries to auto-discover one from the blog homepage
3. If RSS parsing fails and a `scrape_selector` is configured, it falls back to HTML scraping
4. New articles are saved to the database as unread
5. Already-tracked articles are skipped

### Feed Auto-Discovery

BlogWatcher searches for feeds in two ways:

-   Looking for `<link rel="alternate">` tags with RSS/Atom types
-   Checking common feed paths: `/feed`, `/rss`, `/feed.xml`, `/atom.xml`, etc.

### HTML Scraping

When RSS isn't available, provide a CSS selector that matches article links:

```bash
# Example selectors
--scrape-selector "article h2 a"      # Links inside article h2 tags
--scrape-selector ".post-title a"     # Links with post-title class
--scrape-selector "#blog-posts a"     # Links inside blog-posts ID
```

## Database

BlogWatcher stores data in SQLite at `~/.blogwatcher/blogwatcher.db`:

-   **blogs** - Tracked blogs (name, URL, feed URL, scrape selector, topics)
-   **articles** - Discovered articles (title, URL, dates, read status, categories)

Performance indexes are automatically created on `blog_id`, `is_read`/`discovered_date`, and `discovered_date`.

## Development

### Requirements

-   Go 1.24+

### Running Tests

```bash
# Run all tests
go test ./...
```

### Publishing

in addition to publishing to main a new tag should be published so homebrew will get the updated version:
```
  git tag vX.Y.Z
  git push origin vX.Y.Z
```

## License

MIT
