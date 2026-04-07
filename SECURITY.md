# Security Policy

## Supported Versions

We release security updates for the following versions:

| Version | Supported          |
| ------- | ------------------ |
| 1.x.x   | :white_check_mark: |
| < 1.0   | :x:                |

## Reporting a Vulnerability

We take the security of Go SNMP OLT ZTE C320 seriously. If you believe you have found a security vulnerability, please report it to us as described below.

### Reporting Process

**Please DO NOT report security vulnerabilities through public GitHub issues.**

Instead, please report them via email to:
- **Primary Contact**: admin@ckt.co.id
- **Alternative**: Create a [Security Advisory](https://github.com/Cepat-Kilat-Teknologi/go-snmp-olt-zte-c320/security/advisories/new)

You should receive a response within 48 hours. If for some reason you do not, please follow up to ensure we received your original message.

### What to Include

Please include the following information in your report:

- Type of vulnerability
- Full paths of source file(s) related to the manifestation of the issue
- Location of the affected source code (tag/branch/commit or direct URL)
- Any special configuration required to reproduce the issue
- Step-by-step instructions to reproduce the issue
- Proof-of-concept or exploit code (if possible)
- Impact of the issue, including how an attacker might exploit it

### Response Timeline

- **Initial Response**: Within 48 hours
- **Status Update**: Within 7 days
- **Fix Release**: Depends on severity
  - Critical: Within 7 days
  - High: Within 30 days
  - Medium: Within 90 days
  - Low: Next regular release

## Security Best Practices

### Deployment Security

#### 1. Environment Configuration

**CRITICAL - Never commit sensitive data:**
- `.env` files are gitignored - use `.env.example` as template
- Use environment variables for all sensitive configuration
- Rotate credentials regularly

**Example secure configuration:**
```bash
# NEVER commit actual values
SNMP_COMMUNITY=complex_random_string_here
REDIS_PASSWORD=another_complex_password
```

#### 2. HTTPS/TLS

**Always use HTTPS in production:**
```bash
USE_TLS=true
TLS_CERT_FILE=/path/to/fullchain.pem
TLS_KEY_FILE=/path/to/privkey.pem
```

**Obtain certificates from:**
- [Let's Encrypt](https://letsencrypt.org/) (free)
- Commercial CA (Sectigo, DigiCert, etc.)

#### 3. CORS Configuration

**Restrict origins in production:**
```bash
# Bad - allows all origins
CORS_ALLOWED_ORIGINS=*

# Good - specific domains only
CORS_ALLOWED_ORIGINS=https://yourdomain.com,https://api.yourdomain.com
```

#### 4. Network Security

**Firewall rules:**
```bash
# Allow only necessary ports
sudo ufw allow 443/tcp  # HTTPS
sudo ufw deny 8081/tcp  # Block direct access to app port
sudo ufw allow from <trusted_ip> to any port 6379  # Redis (if external)
```

**Use internal networks:**
- Keep Redis on private network
- Use VPN or SSH tunnel for administrative access
- Implement network segmentation

#### 5. SNMP Security

**SNMP v2c limitations:**
- Community strings are transmitted in plaintext
- Use strong, unique community strings
- Consider network-level encryption (VPN/IPSec)

**Network restrictions:**
```bash
# Configure OLT to accept SNMP only from specific IPs
# Check your ZTE C320 OLT documentation
```

### Application Security

#### 1. Rate Limiting

Built-in rate limiting protects against abuse:
- **Default**: 100 requests/second, burst 200
- Adjust in `app/routes.go` if needed

#### 2. Request Timeout

- **Default**: 90 seconds
- Prevents resource exhaustion from slow clients

#### 3. Body Size Limit

- **Default**: 1 MB
- Prevents large payload attacks

#### 4. Input Validation

All user inputs are validated:
- Board ID: Must be 1 or 2
- PON ID: Must be 1-16
- ONU ID: Must be numeric

#### 5. Security Headers

Automatically applied headers:
- `X-Content-Type-Options: nosniff`
- `X-Frame-Options: DENY`
- `X-XSS-Protection: 1; mode=block`
- `Strict-Transport-Security` (when TLS enabled)
- `Content-Security-Policy`

### Redis Security

#### 1. Authentication

**Always set Redis password:**
```bash
REDIS_PASSWORD=very_strong_password_here
```

#### 2. Network Binding

**Bind to localhost if Redis is on same host:**
```bash
# In redis.conf
bind 127.0.0.1 ::1

# OR with Docker
docker run -d redis:7.2 redis-server --protected-mode yes
```

#### 3. Disable Dangerous Commands

```bash
# In redis.conf
rename-command FLUSHDB ""
rename-command FLUSHALL ""
rename-command CONFIG ""
```

### Container Security

#### 1. Use Official Images

Always use official images from trusted sources:
```yaml
services:
  redis:
    image: redis:7.2  # Official Redis image
```

#### 2. Run as Non-Root

Application runs as non-root user in Docker:
```dockerfile
USER nonroot:nonroot
```

#### 3. Read-Only Filesystem

Mount sensitive files as read-only:
```yaml
volumes:
  - ./certs:/certs:ro
```

#### 4. Resource Limits

Set memory and CPU limits:
```yaml
services:
  app:
    mem_limit: 2g
    cpus: '1.0'
```

### Secrets Management

#### 1. Docker Secrets (Docker Swarm)

```yaml
secrets:
  snmp_community:
    external: true
  redis_password:
    external: true

services:
  app:
    secrets:
      - snmp_community
      - redis_password
```

#### 2. Kubernetes Secrets

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: olt-secrets
type: Opaque
stringData:
  snmp-community: "your_snmp_community"
  redis-password: "your_redis_password"
```

#### 3. HashiCorp Vault

For enterprise deployments, integrate with Vault for secret management.

## Known Security Considerations

### 1. SNMP v2c Limitations

- **Issue**: SNMP v2c transmits community strings in plaintext
- **Mitigation**:
  - Use VPN or encrypted network connection to OLT
  - Restrict network access with firewall rules
  - Use strong, unique community strings
  - Consider upgrading to SNMPv3 if OLT supports it

### 2. Cache Timing Attacks

- **Issue**: Cache hit/miss timing could leak information
- **Impact**: Low - only reveals if data exists in cache
- **Mitigation**: Implemented - consistent error responses

### 3. Redis Cache Poisoning

- **Issue**: Compromised Redis could serve malicious data
- **Mitigation**:
  - Secure Redis with authentication
  - Use private network for Redis
  - Implement Redis ACLs if available

### 4. Denial of Service

- **Issue**: High request rate could exhaust resources
- **Mitigation**: Implemented
  - Rate limiting: 100 req/s
  - Request timeout: 90s
  - Body size limit: 1 MB
  - Connection pooling

## Security Checklist

### Deployment Checklist

- [ ] Changed default SNMP community string
- [ ] Set Redis password (REDIS_PASSWORD)
- [ ] Enabled HTTPS/TLS (USE_TLS=true)
- [ ] Configured restrictive CORS origins
- [ ] Disabled unnecessary ports in firewall
- [ ] Set up network segmentation
- [ ] Configured resource limits (memory, CPU)
- [ ] Enabled security headers
- [ ] Set up log aggregation and monitoring
- [ ] Implemented backup strategy
- [ ] Documented incident response plan
- [ ] Tested disaster recovery procedures

### Code Security

- [ ] All inputs validated
- [ ] No hardcoded credentials
- [ ] Secrets loaded from environment variables
- [ ] Error messages don't leak sensitive information
- [ ] Dependencies regularly updated
- [ ] Code reviewed for security issues
- [ ] Unit tests cover security-critical paths
- [ ] SAST (Static Analysis) tools run
- [ ] Dependency vulnerability scanning enabled

## Vulnerability Disclosure Policy

### Responsible Disclosure

We follow responsible disclosure practices:

1. **Reporter submits vulnerability** → Security team acknowledges (48h)
2. **Security team investigates** → Severity assessment (7 days)
3. **Fix developed and tested** → Timeline depends on severity
4. **Security advisory published** → After fix is deployed
5. **Reporter credited** → In advisory (if desired)

### Coordinated Disclosure Timeline

- **Critical vulnerabilities**: 7 days
- **High severity**: 30 days
- **Medium severity**: 90 days
- **Low severity**: Next regular release

We may request an extension if more time is needed for complex fixes.

## Security Updates

### Stay Informed

- **GitHub Releases**: Watch repository for security releases
- **Security Advisories**: Enable GitHub Security Advisories notifications
- **Mailing List**: Subscribe (if available)

### Update Policy

- Security patches released as soon as possible
- Always update to latest version promptly
- Review CHANGELOG for security fixes

## Compliance

### Data Protection

This service processes network device data:
- **Data Types**: ONU identifiers, IP addresses, device status
- **Storage**: Cached in Redis (temporary, 10-minute TTL)
- **Access**: Via authenticated SNMP and Redis
- **Retention**: Cache data expires automatically

### Logging

Logs may contain:
- Request IDs
- IP addresses (not logged by default)
- Error messages

**Do not log:**
- SNMP community strings
- Redis passwords
- User credentials

## Security Tools

### Recommended Security Scanning

**1. Dependency Scanning:**
```bash
# Using govulncheck
go install golang.org/x/vuln/cmd/govulncheck@latest
govulncheck ./...
```

**2. Static Analysis:**
```bash
# Using gosec
go install github.com/securego/gosec/v2/cmd/gosec@latest
gosec ./...
```

**3. Container Scanning:**
```bash
# Using Trivy
trivy image cepatkilatteknologi/snmp-olt-zte-c320:2.1.0
```

## Additional Resources

- [OWASP Top 10](https://owasp.org/www-project-top-ten/)
- [CIS Docker Benchmark](https://www.cisecurity.org/benchmark/docker)
- [NIST Cybersecurity Framework](https://www.nist.gov/cyberframework)
- [Go Security Checklist](https://github.com/guardrailsio/awesome-golang-security)

---

**Last Updated**: 2025-12-15

For security concerns, contact: admin@ckt.co.id (or open a Security Advisory)
