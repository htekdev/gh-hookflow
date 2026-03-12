---
name: hookify-block-sensitive
description: Block creation or editing of sensitive files
enabled: true
event: file
action: block
pattern: \.(env|key|pem|cert)$
---

⚠️ **Sensitive file blocked**

Do not create or edit `.env`, `.key`, `.pem`, or `.cert` files. These files may contain secrets and should not be modified by automated tools.
