# Security Guidelines

## Overview

This application implements several security measures to protect against common vulnerabilities. Follow these guidelines when deploying and maintaining the bot.

## Security Features Implemented

### ✅ Input Validation & Sanitization
- All user inputs are validated and sanitized
- URL validation with domain allowlisting
- File path validation to prevent path traversal
- String length limits to prevent DoS attacks

### ✅ Secure File Operations
- Log files created with secure permissions (0600)
- File path validation to prevent directory traversal
- Database file location validation

### ✅ SQL Injection Prevention
- All database queries use parameterized statements
- No dynamic SQL construction from user input

### ✅ Memory Protection
- Course processing limits to prevent memory exhaustion
- Input size limits for all user data
- Cleanup mechanisms for temporary data

### ✅ Configuration Security
- Environment variables for sensitive data
- Configuration validation on startup
- No hardcoded credentials in source code

## Deployment Security Checklist

### Environment Setup
- [ ] Create `.env` file with secure permissions (0600)
- [ ] Set strong, unique Telegram bot token
- [ ] Validate channel ID format
- [ ] Use secure database file location
- [ ] Configure appropriate log file permissions

### File System Security
```bash
# Set secure permissions for sensitive files
chmod 600 .env
chmod 600 *.db
chmod 600 *.log
```

### Network Security
- [ ] Deploy behind firewall if possible
- [ ] Use HTTPS for all external requests
- [ ] Monitor for unusual traffic patterns
- [ ] Implement rate limiting at network level

### Monitoring & Logging
- [ ] Monitor log files for suspicious activity
- [ ] Set up log rotation to prevent disk exhaustion
- [ ] Monitor memory and CPU usage
- [ ] Track failed authentication attempts

## Configuration Security

### Required Environment Variables
```bash
TELEGRAM_BOT_TOKEN=your_actual_bot_token
TELEGRAM_CHANNEL_ID=@your_channel_id
```

### Optional Security Settings
```yaml
# config.yaml
scraping:
  rate_limit_delay_seconds: 5  # Increase for better rate limiting
  interval_minutes: 60         # Reduce frequency if needed

filters:
  max_courses_per_hour: 5      # Limit course posting rate
```

## Security Limitations & Considerations

### ⚠️ Known Limitations
1. **No User Authentication**: Any user can interact with the bot
2. **Public Channel**: Course information is visible to all channel members
3. **External Dependencies**: Relies on third-party websites for course data

### 🔒 Additional Hardening (Recommended)
1. **User Allowlisting**: Implement user ID validation
2. **API Rate Limiting**: Add Telegram API call limits
3. **Content Filtering**: Implement additional content validation
4. **Monitoring**: Add security event logging

## Incident Response

### If Security Issue Detected
1. **Immediate**: Stop the bot service
2. **Assess**: Check logs for extent of compromise
3. **Isolate**: Revoke bot token if necessary
4. **Clean**: Remove any malicious data
5. **Update**: Apply security patches
6. **Monitor**: Increase logging temporarily

### Emergency Contacts
- Bot Admin: [Your Contact]
- Telegram: @BotFather for token issues

## Security Updates

### Regular Maintenance
- [ ] Update Go dependencies monthly
- [ ] Review access logs weekly  
- [ ] Rotate bot token annually
- [ ] Update allowlisted domains as needed

### Version Control Security
- [ ] Never commit `.env` files
- [ ] Review all commits for credentials
- [ ] Use signed commits when possible
- [ ] Keep security patches up to date

## Reporting Security Issues

If you discover a security vulnerability, please:
1. **Do not** open a public issue
2. Contact the maintainer directly
3. Provide detailed reproduction steps
4. Allow time for patching before disclosure

## Compliance Notes

This application is designed for:
- ✅ Educational use
- ✅ Personal course discovery
- ✅ Information sharing

This application is **NOT** designed for:
- ❌ Automated course enrollment
- ❌ Terms of service violation
- ❌ Commercial exploitation
- ❌ Spam or abuse

---

**Last Updated**: 2025-01-26
**Security Review**: Complete