# envcheck

Checks a `.env` file against `.env.example` before you deploy: missing keys, empty values, keys set twice, and values of the wrong type. It never prints the values, so the output is safe to keep in CI logs.

It's one small binary that uses only the Go standard library.

## Install

```bash
go install github.com/Ifazawezk/envcheck@latest
```

## Example

`.env.example` lists the keys the app needs. A comment after a key says what kind of value it takes:

```bash
# Copy this file to .env and fill in the values.
APP_ENV=development # one of: development,staging,production
PORT=8080 # int
DATABASE_URL= # url
STRIPE_SECRET_KEY=
ADMIN_EMAIL= # email
DEBUG=false # bool
SENTRY_DSN= # optional url
```

Here is a `.env` that has drifted from it:

```bash
APP_ENV=prod
PORT=8080
DATABASE_URL=postgres://app:Xk29fLq7Pw@db.internal:5432/app
STRIPE_SECRET_KEY=
DEBUG=false
REDIS_URL=redis://cache:6379
PORT=80
```

```
$ envcheck
.env:1: error: APP_ENV should be one of development, staging, production (got "****")
.env:4: error: STRIPE_SECRET_KEY is empty
.env:6: warning: REDIS_URL is not in .env.example
.env:7: error: PORT is set more than once (first on line 2)
.env.example:6: error: ADMIN_EMAIL is missing from .env
4 errors, 1 warning
```

Problems go to stdout as `file:line: severity: message`, so editors and CI log viewers can jump to the line. The summary goes to stderr. `SENTRY_DSN` isn't reported because it's marked optional.

## Usage

```bash
envcheck                                   # check .env against .env.example
envcheck -env .env.production              # check a different file
envcheck -env deploy/.env -example deploy/.env.example
envcheck -strict -quiet                    # extra keys fail too, print errors only
envcheck -json > envcheck.json             # machine-readable result
```

| Flag | Default | |
|---|---|---|
| `-env` | `.env` | the file to check |
| `-example` | `.env.example` | the file that lists the expected keys |
| `-strict` | off | keys that are not in the example are errors instead of warnings |
| `-json` | off | print the result as JSON |
| `-quiet` | off | print errors only: no warnings and no summary line |

The exit code is `0` when there are no errors (warnings alone don't fail), `1` when there is at least one error, and `2` when a file can't be read or has a syntax error, or the flags are wrong.

`-json` prints one object with `env`, `example`, `ok`, `errors`, `warnings` and an `issues` array. Each issue has `severity`, `kind` (`missing`, `extra`, `empty`, `duplicate` or `invalid`), `key`, `file`, `line` and `message`, plus the masked `value` for invalid values. With `-quiet`, warnings are left out of `issues` but still counted in `warnings`.

## What counts as a problem

| Problem | Severity |
|---|---|
| key is in `.env.example` but not in `.env` | error, unless the key is marked `# optional` |
| key is in `.env` but not in `.env.example` | warning, or error with `-strict` |
| required key has an empty value | error |
| key is set more than once in either file | error |
| value doesn't match the type hint | error |

For a key set twice, the last value is the one checked, since that's what most loaders use. It's still an error because loaders don't all agree on which one wins.

## Type hints

A hint is a comment at the end of a line in `.env.example`, with a space before the `#`.

| Hint | Accepts |
|---|---|
| `# int` or `# integer` | whole numbers like `8080` or `-1` |
| `# number` or `# float` | numbers like `3`, `-0.5` or `1e-3` |
| `# string` | any text, which is mostly useful as `# optional string` |
| `# bool` or `# boolean` | `true`, `false`, `1`, `0`, `yes`, `no`, `on`, `off`, in any case |
| `# url` | an absolute URL with a scheme and host, like `https://api.example.com` or `postgres://user:pass@db:5432/app`. Files and sockets can leave the host out, as in `sqlite:///data/app.db` or `unix:///var/run/docker.sock`, and `jdbc:` URLs work too |
| `# email` | a plain address like `ops@example.com` |
| `# one of: debug,info,warn,error` | exactly one of the listed values (case sensitive) |
| `# optional` | the key may be missing or empty |
| `# required` | changes nothing, since keys are required unless marked optional, but it can sit next to a type: `# required int` |

`optional` combines with a type: `# optional url`, `# int, optional` or `# optional, one of: eu,us`. Put it before `one of:`, because everything after the colon is read as the list of allowed values. Hint words can be in any case. Empty values are never type checked: a required key is reported as empty and an optional one passes. A comment that isn't made only of hint words, such as `# ask the ops team for this`, is ignored.

## .env syntax

- `KEY=value`, with or without spaces around `=`, and an optional `export ` in front
- full-line `#` comments and blank lines
- inline comments after unquoted values, like `PORT=8080 # the port`. The `#` needs a space before it, so `COLOR=#fff` and `PASS=abc#123` keep the `#`
- single quotes, taken literally: `'no \n escapes here'`
- double quotes with `\n`, `\r`, `\t`, `\"`, `\\` and `\$` escapes
- values in either kind of quotes can span several lines, for private keys and certificates
- Windows line endings and a UTF-8 byte order mark

Keys are letters, digits and underscores and can't start with a digit. Variables like `${HOME}` are not expanded, so a value that uses one, such as `postgres://${DB_USER}@db/app` or `${PORT:-8080}`, skips the type check. Its real value is only known once the loader fills it in. Loaders leave single-quoted values alone, so those are checked as written.

Files must be UTF-8. Windows PowerShell 5.1 writes UTF-16 with `>` and `>>`, and envcheck reports that as a syntax error on the line where it starts, because most loaders can't read it either.

## How values are hidden

Output has key names, file names and line numbers, never values. A value that fails a type check is masked: under 8 characters it becomes `****`, and longer ones keep only their first two characters, like `sk****`. The number of stars is always the same, so the length doesn't leak. Syntax errors don't echo the line, in case a secret was pasted on a line of its own.

Such a pasted secret can parse as a key, since `dGhpc3Rva2Vu==` is valid `KEY=value` syntax. So a name that isn't in `.env.example` is masked too if it's at least 12 characters long and either mixes upper and lower case or has digits but no underscore:

```
.env:6: warning: dG**** is not in .env.example (name hidden, this line looks like a pasted secret)
```

Names like `DATABASE_URL`, `http_proxy` and `S3_BUCKET_NAME_2`, and any name the example lists, are shown as usual. The check only looks at the shape of the name, so a short token, or one in a single case with no digits, still gets printed. Rotate any secret that has reached a log.

## In CI

A GitHub Actions job, with the production `.env` stored as a secret:

```yaml
- uses: actions/setup-go@v5
  with:
    go-version: stable
- run: go install github.com/Ifazawezk/envcheck@latest
- name: Check production env
  env:
    PRODUCTION_ENV: ${{ secrets.PRODUCTION_ENV }}
  run: |
    printf '%s\n' "$PRODUCTION_ENV" > .env.production
    envcheck -env .env.production -strict
```

Or at the top of a deploy script:

```bash
envcheck -strict -quiet || { echo "fix .env before deploying"; exit 1; }
```

## License

MIT, see [LICENSE](LICENSE).
