Your job is to go in and add continuous HTTP health checks to the dashboard here to this Go CLI/TUI app.

Probe each running app's `/up` endpoint on a short interval and show status plus latency on its panel, keeping the last latency visible when an app stops without pretending it is healthy. Deploy-time verification already exists; this is the ongoing version.

Done is when make test runs clean and so do house-style lints

/home/mrjohn/Work/tries/2026-11-03-house-style/house-style
