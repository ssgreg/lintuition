# Security

Please report vulnerabilities privately through GitHub's
[private vulnerability reporting](https://github.com/ssgreg/lintuition/security/advisories/new),
not in a public issue.

Things that matter most here: anything that sends source code, literal values or credentials to a
classifier when the payload policy says it must not; anything that lets text in the linted code
change what lintuition does; and the custom builder running code it was not asked to.
