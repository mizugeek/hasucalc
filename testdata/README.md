# Test fixtures

Unit and regression tests generate their own data in-process and do not need
checked-in workbooks.

Optional large Office/HWK samples for manual I/O verification belong only on
your machine:

```text
testdata/local/
```

Place files such as `sample-5.xlsx` or `Perpetual-Calendar-Version-26-0.ods`
there. That directory is gitignored and must never be published.

Root-level `*.hwk` / `*.xlsx` / `*.png` and similar artifacts are also ignored.
