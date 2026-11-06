Your job is to go in and remove the per-frame allocation and repeated summing from the charts here to this Go CLI/TUI app.

The sliding sum is recomputed from scratch for every point and the padded slices are rebuilt on every render; make those linear and reuse the buffers without changing how the charts look.

Done is when make test runs clean and so do house-style lints

/home/mrjohn/Work/tries/2026-11-03-house-style/house-style
