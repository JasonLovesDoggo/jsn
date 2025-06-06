# portkill

A simple utility to kill processes by port number.

## Usage

```
portkill [options] port [port...]
```

### Options

- `-f`: Force kill the process (SIGKILL instead of SIGTERM)
- `-l`: List processes using the port but don't kill them
- `-v`: Verbose output

### Examples

```bash
# Kill process using port 8080
portkill 8080

# Force kill process using port 3000
portkill -f 3000

# List processes using port 5000 without killing them
portkill -l 5000

# Kill processes using multiple ports
portkill 8080 3000 5000
```

## Alias

This utility can also be invoked with the alias `pk`:

```bash
pk 8080
```

## Environment Variables

All flags can be set using environment variables with the prefix `PORTKILL_`:

- `PORTKILL_F`: Force kill (equivalent to -f)
- `PORTKILL_L`: List only (equivalent to -l)
- `PORTKILL_V`: Verbose output (equivalent to -v)
