# Narrow batched download listing

CQR04 replaces a list's per-descriptor DownloadSource hydration with a bounded set query over descriptor/source/linkage/session rows, using the existing Access SQL conditions. It keeps current owner/device, download grant, age/library/label restrictions (including episode-parent rules), source path/size/mtime, present-file and unsupported multipart/shared-file checks. Counts and pagination remain after filtering so hidden records do not leak through totals. Preparation and single-record access still use full execution-time source checks.

The store list uses two SELECTs independent of descriptor count (current user plus descriptor projection), rather than one list and at least seven more per descriptor. It does not open, hash or probe files, or hydrate item metadata/streams.

Same synthetic router fixture, 100 descriptors, limit=1, seven warm samples: median 31.260 ms / 1.781 MiB allocated before; 0.890 ms / 0.152 MiB after. This is one cloud-process micro-measurement, not a production latency guarantee. Full-page median was 29.523 ms / 1.833 MiB before and 1.152 ms / 0.160 MiB after. Logs retained outside the repository; scratch measurement removed from source.

The access/revision projection is compared with DownloadSource across fifteen isolated visibility/revocation/topology scenarios; database errors remain errors. Go vet, full Go tests with serial packages, and focused API/store download race checks pass. No source media, owner data or schema/API contract changes. Existing request-failure diagnostics remain; successful lists and ordinary hidden records add no log volume or secret data.
