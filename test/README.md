# Integration Tests

## SSH Integration Tests

Tests SSH client functionality against local Docker containers.

### Setup

SSH keys are generated on first run. To regenerate:

```bash
ssh-keygen -t rsa -b 2048 -f testdata/ssh/test_key -N '' -C 'test@benchctl'
cp testdata/ssh/test_key.pub testdata/ssh/authorized_keys
chmod 600 testdata/ssh/test_key
```

### Running Tests

```bash
# Start SSH containers
just compose-up

# Run integration tests
just integration-test

# Cleanup
just compose-down
```

### Test Environment

- 2 SSH containers on ports 2222 and 2223
- User: `testuser`
- Key-based auth only
- Keys in `testdata/ssh/` (gitignored)

