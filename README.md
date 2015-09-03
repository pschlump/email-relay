# Email Relay 

## Overview

email-relay converts HTTPS get requests into SMTP to send email.  A system of templating is provided.
The requests are authorized with either a global auth_token or via an IP/Auth pair that limits the
sending to a specific IP address and authorization token.

All of this is set with configuration files.

At the present time this only responds to GET requests.  Since the auth_token is passed you should
only use /api/send with HTTPS.  The current set of certificates that are used are self-signed.

To generate certatificates:

``` base

$ openssl req -x509 -nodes -days 365 -newkey rsa:2048 -keyout key.pem -out cert.pem

```

This code has not been tested on Windows and I have no intention of porting it to run
on Windows.

## Configuration Files

### .email/email-auth.cfg

Example: 
``` JSON

{
	"HostIP":"",
	"Port":"80",
	"WWWPath":"/home/ubuntu/www/www_default_com",
	"Auth":"mypassword",
	"Cert":"/home/ubuntu/cfg/cert.pem",
	"Key":"/home/ubuntu/cfg/key.pem",
	"MonitorURL": "no",
	"DebugEmailAddr": "pschlump@yahoo.com",
	"ApprovedApps": { "content-pusher" : "yes" }
}

```

This is the primary configuration file.  If *DebugEmailAddr* is set to an address then all email
will be sent to that address.  The list of valid apps for templates is specified with *ApprovedApps*.

To use per-ip authorization set *Auth* to "per-ip" then

``` JSON

	"IPAuth" : { "127.0.0.1": "password1", "12.221.114.18": "password2" },

```

It will check that the requesting IP is in the set and that the auth_token matches the requried
password.


### .email/email-config.json

This file has the setup for the email gateways - the SMTP configuration.

Example: 
``` JSON

{
	 "Username":"emailusername"
	,"Password":"smtppassword"
	,"EmailServer":"email-smtp.yourdomain.com"
	,"Port":587
}

```

### &lt;some-path&gt;/cert.pem and &lt;some-path&gt;/key.pem

If you supply these then HTTPS will be supported.   The files can be in the directory where the
tool is run or use a hard path to them as show in the example above.


