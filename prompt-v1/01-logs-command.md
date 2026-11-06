Your job is to go in and add a `once logs <host>` command here to this Go CLI/TUI app.

Logs already stream inside the TUI through the existing log streamer; reuse that so the command follows the app container, supports `--tail` and `--grep`, and exits cleanly on ctrl+c.

Done is when make test runs clean and so do house-style lints

/home/mrjohn/Work/tries/2026-11-03-house-style/house-style
