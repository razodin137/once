Your job is to go in and make the dashboard render incrementally here to this Go CLI/TUI app.

It currently rebuilds every panel and the whole viewport on every tick; re-render only the panels whose data or selection changed, cache the static chrome and scales, and keep clicks and scrolling landing on the right rows.

Done is when make test runs clean and so do house-style lints

/home/mrjohn/Work/tries/2026-11-03-house-style/house-style
