Your job is to go in and serve a deployed folder from a tiny static container here to this Go CLI/TUI app.

Keep hosts, TLS, health checks and zero-downtime swaps identical to an app container; only the image and the mount differ, so a folder deploys through the same path as everything else.

Done is when make test runs clean and so do house-style lints

/home/mrjohn/Work/tries/2026-11-03-house-style/house-style
