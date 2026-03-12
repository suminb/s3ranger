# S3Ranger

A terminal-based file manager for Amazon S3 and S3-compatible services. Built with Go using [Bubble Tea](https://github.com/charmbracelet/bubbletea) and [Lip Gloss](https://github.com/charmbracelet/lipgloss).

## Features

- Browse and navigate S3 buckets and objects
- Upload and download files and directories with progress tracking
- Copy and move files across buckets
- Delete, rename with confirmation prompts
- Overwrite protection for downloads
- Multi-select for batch operations
- Bucket filtering and object sorting
- Pagination with progressive loading
- 4 themes: GitHub Dark, Dracula, Solarized, Sepia
- S3-compatible services support (MinIO, LocalStack, etc.)

## Build

```bash
go build -o s3ranger ./
```

## Usage

```bash
# Launch with default AWS credentials
./s3ranger

# Use a specific AWS profile
./s3ranger --profile-name my-profile

# Use a custom S3-compatible endpoint
./s3ranger --endpoint-url http://localhost:9000

# Interactive configuration wizard
./s3ranger configure
```

## CLI Flags

| Flag | Description |
|---|---|
| `--profile-name` | AWS profile name |
| `--aws-access-key-id` | AWS access key ID |
| `--aws-secret-access-key` | AWS secret access key |
| `--aws-session-token` | AWS session token |
| `--region-name` | AWS region |
| `--endpoint-url` | Custom S3 endpoint URL |
| `--theme` | UI theme |
| `--config` | Path to config file (default `~/.s3ranger.config`) |
| `--download-directory` | Default download directory |
| `--enable-pagination` | Enable pagination |
| `--disable-pagination` | Disable pagination |
| `--version` | Show version |

## Key Bindings

| Key | Action |
|---|---|
| `Tab` | Switch panels |
| `Enter` | Open bucket / navigate into folder |
| `Backspace` | Navigate up |
| `Space` | Toggle selection |
| `Ctrl+A` | Select / deselect all |
| `u` | Upload |
| `d` | Download |
| `x` | Delete |
| `r` | Rename |
| `m` | Move / Copy |
| `s` | Sort |
| `t` | Cycle theme |
| `?` | Help |
| `Ctrl+Q` | Quit |

## Configuration

Create `~/.s3ranger.config` manually or via `s3ranger configure`:

```toml
[default]
profile_name = "my-profile"
theme = "Dracula"
enable_pagination = true
download_directory = "~/Downloads"
```
