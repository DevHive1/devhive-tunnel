# 📘 دليل البناء والتشغيل الكامل — devhive-tunnel

دليل عملي مفصّل من الصفر: بناء المنصة، تشغيل الخادم (Hub)، إنشاء شبكة، دعوة الأعضاء،
الدردشة ومشاركة الملفات، لوحة التحكم، وضع LAN، وربط وكلاء الذكاء الاصطناعي (MCP).

> **الخلاصة السريعة:** المنصة تتكوّن من **Hub** (خادم تنسيق صغير) + تطبيق **devtun** على كل
> جهاز. أنت تُشغّل الـ Hub بنفسك (على حاسوب أو VPS أو هاتف)، ثم تُنشئ شبكة مشفّرة وتدعو
> من تريد. لا حسابات ولا سحابة طرف ثالث.

---

## المحتويات

1. [نظرة عامة على المكوّنات](#1-نظرة-عامة-على-المكوّنات)
2. [المتطلبات](#2-المتطلبات)
3. [بناء المنصة](#3-بناء-المنصة)
4. [تشغيل الـ Hub](#4-تشغيل-الـ-hub)
5. [إعداد جهازك (العميل)](#5-إعداد-جهازك-العميل)
6. [إنشاء شبكة ودعوة الأعضاء](#6-إنشاء-شبكة-ودعوة-الأعضاء)
7. [الاستخدام اليومي: دردشة وملفات](#7-الاستخدام-اليومي-دردشة-وملفات)
8. [لوحة التحكم (المالك و GM)](#8-لوحة-التحكم-المالك-و-gm)
9. [وضع LAN والألعاب](#9-وضع-lan-والألعاب)
10. [اللغة العربية](#10-اللغة-العربية)
11. [ربط وكلاء الذكاء الاصطناعي (MCP)](#11-ربط-وكلاء-الذكاء-الاصطناعي-mcp)
12. [سيناريوهات جاهزة خطوة بخطوة](#12-سيناريوهات-جاهزة-خطوة-بخطوة)
13. [استكشاف الأخطاء](#13-استكشاف-الأخطاء)
14. [الأمان والنسخ الاحتياطي](#14-الأمان-والنسخ-الاحتياطي)
15. [ملخص الأوامر](#15-ملخص-الأوامر)
16. [بناء الإصدارات والتطوير](#16-بناء-الإصدارات-والتطوير)
17. [مسارات الملفات المهمة](#17-مسارات-الملفات-المهمة)
18. [ملاحظات خاصة ببيئتك الحالية (Termux + Ubuntu PRoot)](#18-ملاحظات-خاصة-ببيئتك-الحالية)

---

## 1. نظرة عامة على المكوّنات

```
                 ┌──────────────────────────────┐
                 │        devhive-hub           │
                 │  • تنسيق الأعضاء              │
                 │  • مصادقة (Argon2id)          │
                 │  • دعوات/حظر/تدقيق            │
                 │  • لوحة ويب للمالك و GM       │
                 └──────────────┬───────────────┘
        TCP 7676 (تحكّم مشفّر)  │  HTTP/HTTPS 7600 (لوحة)
        ┌───────────────┬───────┴───────┬───────────────┐
        │               │               │               │
   devtun (PC)     devtun (هاتف)   devtun (لابتوب)   dh-mcp (وكيل AI)
   دردشة/ملفات      دردشة/ملفات      دردشة/ملفات       أدوات MCP
   + TUN (LAN)     بدون TUN
```

| المكوّن | الملف التنفيذي | الوظيفة |
|---|---|---|
| Hub | `devhive-hub` | خادم التنسيق + لوحة الويب. لا يرى محتوى الدردشة/الملفات (كل شيء E2E). |
| العميل | `devtun` | معالج إعداد، إنشاء/انضمام، دردشة، ملفات، وضع LAN. |
| MCP | `dh-mcp` | يمنح وكلاء الذكاء الاصطناعي أدوات للشبكات والدردشة والملفات. |

**أدوار الصلاحيات:**

- **المالك (Owner):** أنشأ الشبكة. يملك *مفتاح الإدارة* (Admin Key) الذي يفتح لوحة التحكم،
  ويُنشئ الدعوات، ويحظر الأعضاء، ويبدّل المفتاح.
- **العضو (Member):** دردشة، رسائل خاصة، ملفات، وLAN حسب المنصة. لا يملك لوحة تحكم.
- **GM (مشرف عام):** مفتاح على مستوى الـ Hub كله: حظر عالمي، إيقاف شبكات، إحصاءات، تدقيق.
  لا يرى المحتوى المشفّر.

---

## 2. المتطلبات

### للبناء (مرة واحدة)

- **Go 1.22 أو أحدث** (تم الاختبار على 1.26). لا حاجة لأدوات C.
- مساحة قرص: ~500MB أثناء البناء (يمكن تنظيف الكاش لاحقًا).
- Git (لجلب المشروع إن لم يكن موجودًا).

### للاستخدام

- **الـ Hub**: أي جهاز يمكن للأعضاء الوصول إليه عبر TCP.
  - على نفس شبكة Wi-Fi: يكفي عنوانه المحلي (مثل `192.168.1.10`).
  - عن بُعد: VPS بعنوان عام + فتح منفذ `7676` (ومنفذ `7600` للوحة).
- **العميل**: Windows / Linux / Termux (Android). لا يحتاج صلاحيات مدير إلا لوضع LAN على
  سطح المكتب.

### المنافذ الافتراضية

| المنفذ | البروتوكول | الغرض | ملاحظات |
|---|---|---|---|
| `7676` | TCP | بروتوكول التحكّم (بيانات مشفّرة) | يجب أن يصل إليه كل الأعضاء. |
| `7600` | TCP HTTP | لوحة الويب | يمكن حصره على `127.0.0.1` خلف وكيل عكسي. |

---

## 3. بناء المنصة

### 3.1 الطريقة الأسهل (Makefile)

```bash
cd devhive-tunnel
make build
```

الناتج:

```
bin/devtun
bin/devhive-hub
bin/dh-mcp
```

### 3.2 بناء يدوي

```bash
cd devhive-tunnel
go build -trimpath -o bin/devhive-hub ./cmd/devhive-hub
go build -trimpath -o bin/devtun      ./cmd/devtun
go build -trimpath -o bin/dh-mcp      ./cmd/dh-mcp
```

### 3.3 بناء إصدارات لمنصات أخرى

```bash
# Windows 64-bit
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o devtun.exe ./cmd/devtun

# Linux ARM64 (VPS/Raspberry Pi)
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o devtun-arm64 ./cmd/devtun

# Android/Termux ARM64
GOOS=android GOARCH=arm64 CGO_ENABLED=0 go build -o devtun-android ./cmd/devtun
```

أو استخدم السكربت الجاهز الذي يبني الحزم الثلاث لكل الأهداف:

```bash
scripts/build-all.sh          # الكل: linux/amd64, linux/arm64, windows/amd64, android/arm64
# عند ضيق المساحة يمكن اختيار أهداف محددة:
TARGETS="windows/amd64 android/arm64" scripts/build-all.sh
```

الناتج في `dist/devhive-tunnel-<version>-<os>-<arch>/`.

### 3.4 Termux (بدون root)

```bash
pkg update -y && pkg install -y golang git
cd devhive-tunnel
scripts/termux-bootstrap.sh
```

السكربت يبني لـ `android/arm64` وينسخ الثنائيات إلى `$PREFIX/bin` لتعمل `devtun` من أي مكان.

> **ملاحظة:** الدردشة/الملفات تعمل بالكامل على Termux. وضع LAN الحقيقي يحتاج `/dev/net/tun`
> (جهاز معمّق/root أو proot مع صلاحيات TUN) — وإلا استخدم الدردشة والملفات.

---

## 4. تشغيل الـ Hub

### 4.1 التشغيل الأساسي

```bash
bin/devhive-hub
```

في أول تشغيل سيُنشئ:

- `~/.devhive-hub/hub.json` — حالة الـ Hub (شبكات، حظر، تدقيق، تجزئات المفاتيح).
- `~/.devhive-hub/GM-KEY.txt` — **مفتاح GM**. يُطبع مرة واحدة في السجل أيضًا.

مثال سجل:

```
[hub] GM superuser key created. Store it safely now;
[hub] it is also written to /home/you/.devhive-hub/GM-KEY.txt
[hub] GM key: XXXXXXXXXXXXXXXX
[hub] dashboard: http://[::]:7600
[hub] control listening on [::]:7676
```

> ✋ **انسخ مفتاح GM واحفظه الآن**. من يملكه يستطيع إيقاف الشبكات وحظر الأجهزة عالميًا.

### 4.2 خيارات التشغيل

```bash
bin/devhive-hub \
  -dir ~/.devhive-hub \        # مجلد الحالة
  -listen :7676 \              # عنوان التحكّم (كل الواجهات)
  -web 127.0.0.1:7600 \        # لوحة الويب (محلي فقط، الأفضل خلف وكيل عكسي)
  -subnet 100.72.0.0/16 \      # النطاق الافتراضي للشبكات الجديدة
  -tls-cert /path/fullchain.pem \
  -tls-key  /path/privkey.pem  # HTTPS للوحة (اختياري)
```

| الخيار | الافتراضي | الشرح |
|---|---|---|
| `-dir` | `~/.devhive-hub` | مجلد الحالة (احتفظ بنسخة احتياطية منه). |
| `-listen` | `:7676` | عنوان TCP للتحكّم. استخدم `:7676` للسماح بالاتصالات الخارجية. |
| `-web` | `:7600` | لوحة الويب. `127.0.0.1:7600` + وكيل عكسي = أفضل أمان. |
| `-subnet` | `100.72.0.0/16` | يُستخدم للشبكات التي لا تحدد نطاقًا بنفسها. |
| `-tls-cert` / `-tls-key` | فارغ | يفعّل HTTPS للوحة فقط (بروتوكول التحكّم له تشفيره الخاص). |

فحص الصحة:

```bash
curl http://127.0.0.1:7600/healthz
```

### 4.3 على VPS (Ubuntu/Debian) مع systemd

```bash
sudo useradd -r -s /usr/sbin/nologin devhive
sudo mkdir -p /opt/devhive /var/lib/devhive-hub
sudo cp bin/devhive-hub bin/devtun /opt/devhive/
sudo chown -R devhive:devhive /opt/devhive /var/lib/devhive-hub
sudo tee /etc/systemd/system/devhive-hub.service >/dev/null <<'EOF'
[Unit]
Description=devhive-tunnel hub
After=network-online.target
Wants=network-online.target

[Service]
User=devhive
Group=devhive
ExecStart=/opt/devhive/devhive-hub -dir /var/lib/devhive-hub -listen :7676 -web 127.0.0.1:7600
Restart=on-failure
RestartSec=3
NoNewPrivileges=true
ProtectSystem=strict
ReadWritePaths=/var/lib/devhive-hub
PrivateTmp=true

[Install]
WantedBy=multi-user.target
EOF
sudo systemctl daemon-reload
sudo systemctl enable --now devhive-hub
sudo journalctl -u devhive-hub -f     # لمشاهدة مفتاح GM أول مرة
```

ثم افتح الجدار الناري:

```bash
sudo ufw allow 7676/tcp
# للوحة عبر HTTPS مباشرة: sudo ufw allow 443/tcp ومرّر -tls-cert/-tls-key
```

### 4.4 على Windows

1. شغّل `devhive-hub.exe` (سيظهر تنبيه جدار الحماية → اسمح للشبكات الخاصة/العامة حسب حاجتك).
2. أو أضف القواعد يدويًا في موجه الأوامر كمسؤول:

```bat
netsh advfirewall firewall add rule name="devhive-hub 7676" dir=in action=allow protocol=TCP localport=7676
netsh advfirewall firewall add rule name="devhive-hub 7600" dir=in action=allow protocol=TCP localport=7600
```

### 4.5 على Termux (هاتف كخادم)

```bash
termux-wake-lock              # يمنع النظام من إيقاف العملية
devhive-hub -listen :7676 -web :7600
```

> الهاتف خلف شبكة الجوال غالبًا غير قابل للوصول من الخارج. للاستخدام البعيد استخدم VPS،
> أو أنفاق TCP (مثل Tailscale/SSH tunnel) بحذر.

### 4.6 لماذا تحتاج Hub أصلًا؟

الـ Hub ينسّق الاكتشاف والتوجيه بين الأعضاء (رسائل/ملفات/حزم IP تُمرَّر عبره مشفّرة). هذا
يجعله يعمل خلف NAT دون فتح منافذ على أجهزة الأعضاء — لكنه يعني أن حركة LAN تمر عبر الـ Hub
أيضًا، فاختر خادمًا قريبًا جغرافيًا لتقليل التأخير.

---

## 5. إعداد جهازك (العميل)

### 5.1 المعالج التفاعلي (موصى به لأول مرة)

```bash
devtun setup
```

يسألك:

1. **اللغة**: `1` إنجليزي أو `2` عربي.
2. **عنوان الـ Hub**: مثل `192.168.1.10:7676` أو `my-vps.example.com:7676`.
3. **اسمك المستعار**: يظهر للأعضاء (مثل `ahmed`).
4. **إنشاء أم انضمام**:
   - إنشاء → ينتقل إلى معالج الإنشاء (اسم، ملف أمان، عدد أعضاء، LAN).
   - انضمام → اسم/معرّف الشبكة + كلمة المرور (+ مفتاح دعوة إن لزم).

يُحفظ كل شيء في `~/.devhive-tunnel` (أو `%APPDATA%\devhive-tunnel` على Windows).

### 5.2 الواجهة الكاملة (TUI)

```bash
devtun tui
```

- معالج مرئي خطوة بخطوة، ثم شاشة دردشة حيّة مع قائمة الأعضاء.
- أوامر داخل الشاشة: اكتب في الأسفل ثم Enter.
  - `/dm <peer> <نص>` رسالة خاصة
  - `/send <peer> <ملف>` إرسال ملف
  - `/invite` إنشاء دعوة (للمالك)
  - `/peers` عرض الأعضاء
  - `/quit` خروج

### 5.3 الإعداد بالذكاء الاصطناعي المدمج

```bash
devtun ai
```

محرّك قواعد محلي (بدون API خارجي) يختار لك:

- نطاقًا افتراضيًا (`100.x.0.0/16`)،
- كلمة مرور قوية قابلة للكتابة،
- سياسة الدعوات، حد الأعضاء، حدود التمرير،
- تقييم أمان + أسباب + قائمة تحقق (بالعربية أو الإنجليزية).

الملفات الجاهزة (Profiles):

| الملف | الدعوة | الظهور العام | حد الأعضاء | معدل الانضمام |
|---|---|---|---|---|
| `private` | إلزامية | لا | ~عدد الأعضاء | 10/دقيقة |
| `team` | إلزامية | لا | الأعضاء + 50% | 10/دقيقة |
| `community` | إلزامية | نعم (اختياري) | ضعف الأعضاء | 6/دقيقة |
| `paranoid` | إلزامية | لا | صغير (3+) | 3/دقيقة |

### 5.4 الإنشاء المباشر بالأعلام

```bash
devtun create \
  --hub 192.168.1.10:7676 \
  --name my-hive \
  --profile private \
  --members 6 \
  --password 'My-Strong-Pass-2026' \
  --nick ahmed \
  --yes
```

- إن حذفت `--password` سيقترح النظام واحدًا ويطبعه **مرة واحدة**.
- الأمر يطبع: **Network ID** و**Admin Key** وكلمة المرور. احفظها فورًا.
- `--lan` يفعّل تخطيط LAN (النطاق + إعدادات البث) في خطة الشبكة.
- `--invite` يفرض مفتاح دعوة، و`--anonymous` يسمح بالظهور في القائمة العامة.

---

## 6. إنشاء شبكة ودعوة الأعضاء

### 6.1 من جهة المالك

بعد الإنشاء تصبح عضوًا مالكًا. أنشئ دعوة:

```bash
devtun invite                 # دعوة واحدة صالحة 7 أيام
devtun invite --uses 5 --days 30 --note "فريق العمل"
devtun invite --role admin --uses 1   # دعوة بصلاحيات إدارة (استخدمها بحذر)
```

الناتج:

```
invite key: Ab3xY9Q2Lm7Np1R
```

> الدعوة **أحادية الاستخدام** افتراضيًا. الجهاز الذي انضم مرة يصبح «مُسجّلًا» ويستطيع
> إعادة الاتصال لاحقًا بكلمة المرور فقط، بينما تحتاج الأجهزة الجديدة إلى دعوة.

### 6.2 من جهة العضو الجديد

```bash
devtun join --hub 192.168.1.10:7676 \
  --password 'My-Strong-Pass-2026' \
  --admin-key 'Ab3xY9Q2Lm7Np1R' \
  --nick sara \
  my-hive
```

- يمكن استخدام اسم الشبكة أو معرّفها (`dh-XXXXXXXX`).
- بعد نجاح الانضمام يُحفظ كل شيء، وتظهر قائمة الأعضاء وعنوانك الافتراضي على الشبكة
  (`100.x.x.x`).
- `--stay` يُبقي الاتصال مفتوحًا ويطبع الأحداث (بديل: `devtun listen`).

### 6.3 استعراض الشبكات

```bash
devtun list                     # الشبكات العامة فقط
devtun status                   # هويتك + شبكاتك المحفوظة + الـ Hub
```

---

## 7. الاستخدام اليومي: دردشة وملفات

### دردشة جماعية / خاصة

```bash
devtun chat my-hive
```

```
> مرحبًا بالجميع
> /dm sara هذه رسالة خاصة
> /send sara ./report.pdf
> /invite
> /peers
> /quit
```

### استقبال في الخلفية

```bash
devtun listen my-hive
```

- تُحفظ الملفات الواردة في:
  - Linux/Termux: `~/.devhive-tunnel/downloads/`
  - Windows: `%APPDATA%\devhive-tunnel\downloads\`
- كل ملف يُتحقق من بصمته SHA-256 بعد التجميع. لا يتم حفظ ملف ناقص/تالف.

### إرسال ملف من سطر الأوامر

```bash
devtun send sara ./video.mp4 my-hive
```

يجب أن يكون الطرف الآخر متصلًا (`chat` أو `listen`). التشفير ثنائي (X25519 + XChaCha20)
ولكل ملف بصمة تحقق.

---

## 8. لوحة التحكم (المالك و GM)

### 8.1 دخول المالك

افتح المتصفح على:

```
http://<عنوان-الـ-Hub>:7600
```

- **المعرّف**: معرّف الشبكة (`dh-XXXXXXXX`) — أو اسمها.
- **المفتاح**: Admin Key الذي ظهر عند الإنشاء.

تستطيع من اللوحة:

- رؤية الأعضاء وحالتهم وعناوينهم الافتراضية،
- إنشاء/إلغاء الدعوات،
- حظر/فك حظر/طرد الأعضاء،
- ترقية عضو إلى إدارة،
- عرض سجل التدقيق،
- تعديل إعدادات الشبكة (حد الأعضاء، التمرير، الدعوات…)،
- **تبديل مفتاح الإدارة** بعد مغادرة أي عضو.

### 8.2 دخول GM

نفس العنوان، اختر «GM» وأدخل **مفتاح GM** (من `GM-KEY.txt`). تستطيع:

- عرض كل الشبكات والإحصاءات والتحليلات،
- حظر/فك حظر عالمي، طرد عضو من أي شبكة،
- إيقاف/استئناف شبكة كاملة، حذف شبكة،
- تدقيق شامل وتبديل مفتاح إدارة أي شبكة.

> GM لا يرى محتوى الدردشة أو الملفات — البيانات مشفّرة طرفًا لطرف.

### 8.3 HTTPS

للاستخدام على الإنترنت، شغّل الـ Hub بـ `-tls-cert` و`-tls-key`، أو ضع Nginx/Caddy أمام
المنفذ 7600. لا تعرّض اللوحة على HTTP عادي للإنترنت.

---

## 9. وضع LAN والألعاب

### 9.1 Linux (حقيقي TUN)

```bash
# يتطلب root أو CAP_NET_ADMIN ووجود /dev/net/tun
sudo devtun lan my-hive
```

- ينشئ واجهة (افتراضيًا `devhive0` بمقياس MTU 1400)، يعيّن عنوانك `100.x.x.x/16`،
  ويضيف مسار الشبكة تلقائيًا عبر `ip`.
- اختبر الوصول: `ping 100.x.x.y` (عنوان العضو الآخر من `devtun join` أو `/peers`).
- خيارات: `--dev اسم` لتسمية الواجهة، `--no-config` إذا أردت ضبط العناوين يدويًا.
- التحقق من الواجهة: `ip addr show devhive0` ثم `ip route`.

### 9.2 Windows (Wintun)

1. نزّل `wintun.dll` من <https://www.wintun.net/> (اختر إصدار amd64).
2. ضعه بجانب `devtun.exe` (أو في `PATH`).
3. شغّل موجه الأوامر **كمسؤول**:

```bat
devtun.exe join --hub 203.0.113.10:7676 --password "..." --nick pc my-hive
devtun.exe lan my-hive
```

السكربت يضبط عنوان IP والمسار عبر `netsh` لواجهة باسم `devhive-tunnel`.

### 9.3 ما الذي يعمل وما لا يعمل؟

- يعمل: ألعاب/تطبيقات تستخدم UDP Broadcast أو Multicast للاكتشاف المحلي (معظم الألعاب
  الحديثة)، والاتصال المباشر بعنوان `100.x.x.x`.
- لا يعمل: بروتوكولات الطبقة الثانية الخام (IPX/الألعاب القديمة جدًا) — النفق طبقة ثالثة (IP).
- التأخير يعتمد على المسافة إلى الـ Hub لأن الحزم تُمرَّر عبره (لا يوجد P2P مباشر).
- Termux/Android: بدون root لا يوجد TUN؛ استخدم الدردشة/الملفات. (تطبيق Android أصلي
  يستطيع استخدام VpnService لاحقًا.)

---

## 10. اللغة العربية

- **اللوحة**: زر تبديل اللغة (EN/العربية) مع اتجاه RTL كامل.
- **المعالج TUI/setup**: اختر `2` للعربية؛ تُحفظ في `config.json`.
- **محرّك AI**: يعطي الأسباب وقائمة التحقق بالعربية عند اختيار اللغة العربية.
- **طرفية Android/Termux**: قد لا تشكّل الحروف العربية بشكل مثالي في بعض الطرفيات؛
  اللوحة في المتصفح أفضل للعرض العربي.

---

## 11. ربط وكلاء الذكاء الاصطناعي (MCP)

`dh-mcp` يخاطب بروتوكول MCP عبر stdio. أمثلة إعداد:

**Claude Desktop / Cursor:**

```json
{
  "mcpServers": {
    "devhive-tunnel": {
      "command": "/absolute/path/to/dh-mcp"
    }
  }
}
```

**opencode** (`opencode.json`):

```json
{
  "mcp": {
    "devhive-tunnel": {
      "type": "local",
      "command": ["/absolute/path/to/dh-mcp"],
      "enabled": true
    }
  }
}
```

الأدوات المتاحة للوكيل:

| الأداة | الوظيفة |
|---|---|
| `dh_status` | الهوية والـ Hub والشبكات المحفوظة |
| `dh_networks_list` | قائمة الشبكات |
| `dh_ai_plan` | توليد خطة شبكة (بدون إنشاء) |
| `dh_network_create` | إنشاء شبكة (يعيد معرّفها ومفتاح الإدارة مرة واحدة) |
| `dh_network_join` | الانضمام إلى شبكة |
| `dh_peers` | قائمة الأعضاء |
| `dh_chat_send` / `dh_chat_read` | إرسال/قراءة الدردشة |
| `dh_dm_send` | رسالة خاصة |
| `dh_file_send` | إرسال ملف |
| `dh_invite_create` | إنشاء دعوة |

> الوكيل يستخدم نفس ملفات `~/.devhive-tunnel` الخاصة بمستخدم التشغيل. إن شغّلت الوكيل
> كمستخدم آخر (خدمة/حاوية)، انسخ/اضبط مسار `HOME` أو ملفات الإعداد المناسبة.

اختبار سريع يدوي:

```bash
printf '%s\n' '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}' | dh-mcp
```

---

## 12. سيناريوهات جاهزة خطوة بخطوة

### السيناريو أ — تجربة كاملة على جهاز واحد (بدون VPS)

افتح **3 طرفيات** في مجلد المشروع:

```bash
# الطرفية 1: الـ Hub
cd devhive-tunnel
bin/devhive-hub
```

```bash
# الطرفية 2: أنشئ هوية "أحمد" وشبكة
mkdir -p /tmp/alice
HOME=/tmp/alice bin/devtun create \
  --name my-hive --password 'My-Pass-1234' \
  --profile private --members 4 --nick ahmed --yes
# انسخ Network ID و Admin Key من الناتج

# أنشئ دعوة
HOME=/tmp/alice bin/devtun invite
```

```bash
# الطرفية 3: انضم "سارة" عبر الدعوة
mkdir -p /tmp/sara
HOME=/tmp/sara bin/devtun join \
  --password 'My-Pass-1234' \
  --admin-key '<الصق-كود-الدعوة>' \
  --nick sara my-hive
```

```bash
# الطرفية 3 (سارة تستقبل)
HOME=/tmp/sara bin/devtun listen --nick sara my-hive
# الطرفية 2 (أحمد يرسل)
HOME=/tmp/alice bin/devtun chat --nick ahmed my-hive
# اكتب رسالة ثم:
HOME=/tmp/alice bin/devtun send sara ./README.md my-hive
```

> هذه الطريقة تعمل حتى بدون تغيير المنافذ (الافتراضي `127.0.0.1:7676`)؛ مثالية للتعلم
> والاختبار.

### السيناريو ب — عائلة/أصدقاء على نفس Wi-Fi

1. شغّل الـ Hub على الحاسوب الأقوى: `bin/devhive-hub -listen :7676 -web :7600`.
2. اعرف عنوانه المحلي: `ip addr` (مثال `192.168.1.10`).
3. على كل جهاز:
   ```bash
   devtun join --hub 192.168.1.10:7676 --password '...' --admin-key '<invite>' --nick <اسم> <شبكة>
   ```
4. لوحة المالك من أي متصفح: `http://192.168.1.10:7600`.

> إن حظر جدار الحماية الاتصال: افتح TCP 7676 (و7600 محليًا فقط إن أردت).

### السيناريو ج — فريق عن بُعد (VPS)

1. اتبع قسم [systemd](#43-على-vps-ubuntu-debian-مع-systemd).
2. اشترِ نطاقًا (اختياري) وشغّل HTTPS للوحة عبر `-tls-cert/-tls-key`.
3. وزّع `--hub your-domain:7676` مع كلمة المرور **عبر قناة آمنة**، ودعوة لكل زميل.
4. راجع لوحة المالك أسبوعيًا؛ بدّل مفتاح الإدارة عند مغادرة أي عضو.

### السيناريو د — لاعب LAN

1. أنشئ شبكة مع `--lan` أو من المعالج (LAN games = yes).
2. على كل حاسوب سطح مكتب: `sudo devtun lan <الشبكة>` (Windows: كمسؤول + wintun.dll).
3. افتح اللعبة؛ يجب أن تظهر غرف LAN خلال ثوانٍ (بث الاكتشاف يُنسخ لكل الأعضاء).
4. إن لم تظهر، جرّب الاتصال المباشر بعنوان `100.x.x.x` للاعب المضيف.

### السيناريو هـ — من الهاتف فقط

- **هاتف كعميل:** `devtun setup` ثم دردشة/ملفات (بدون LAN).
- **هاتف كخادم:** `termux-wake-lock && devhive-hub` (يصلح لشبكة محلية/نفق).
- **هاتف كطرف لوحة تحكم:** افتح لوحة الـ Hub في المتصفح — مُحسّنة للجوال.

---

## 13. استكشاف الأخطاء

| الرسالة | السبب | الحل |
|---|---|---|
| `hub unreachable` | الـ Hub متوقف/عنوان خاطئ/جدار حماية | شغّل الـ Hub، تحقق من `IP:7676`، افتح المنفذ |
| `wrong network name, id or password` | بيانات خاطئة | تحقق من كلمة المرور/الاسم؛ الدعوة تُمرَّر في `--admin-key` |
| `invite_required` | الشبكة تتطلب دعوة/مفتاح إدارة | اطلب دعوة جديدة من المالك (`devtun invite`) |
| `bad_password` | كلمة المرور غير مطابقة | أعد المحاولة بدقة (حساسة لحالة الأحرف) |
| `name_taken` | اسم الشبكة مستخدم على نفس الـ Hub | اختر اسمًا آخر أو انضم للشبكة القائمة |
| `network_full` | بلغت الشبكة حد الأعضاء | عدّل `max_peers` من لوحة المالك أو أنشئ خطة أكبر |
| `rate_limited` | محاولات انضمام كثيرة | انتظر دقيقة (لكل شبكة حد حسب الخطة 10/6/3، وسقف عام 12 محاولة/دقيقة لكل عنوان) |
| `banned` / `gm_banned` | حظر من المالك أو GM | تواصل مع المالك/GM لفك الحظر |
| `network_suspended` | GM أوقف الشبكة مؤقتًا | تواصل مع مشرف الـ Hub |
| `no /dev/net/tun` | LAN غير متاح (Termux/حاوية) | استخدم الدردشة/الملفات أو root/proot مع TUN |
| `wintun: cannot create adapter` | `wintun.dll` مفقود أو ليست مسؤولًا | نزّل DLL بجانب exe وشغّل كمسؤول |
| `activation failed` / `ip: ...` في LAN | صلاحيات/أداة `ip` مفقودة | شغّل root، أو اضبط العناوين يدويًا (`--no-config`) |
| اللوحة لا تفتح | منفذ `-web` أو جدار حماية | جرّب `curl http://HUB:7600/healthz` وتحقق من `-web` |
| نسيان مفتاح GM | — | من `~/.devhive-hub/GM-KEY.txt`؛ وإلا استعد نسخة احتياطية (لا توجد إعادة تعيين) |
| نسيان مفتاح الإدارة | — | لا يمكن استرجاعه؛ أنشئ شبكة جديدة أو استخدم GM لتبديله إن أمكن |
| `no space left on device` | كاش Go / قرص ممتلئ | `go clean -cache` ثم أعد البناء |
| انقطاع متكرر | شبكة غير مستقرة/الـ Hub يعيد التشغيل | أعد تشغيل العميل؛ راقب سجل الـ Hub |

**جمع معلومات التشخيص:**

```bash
devtun status
bin/devhive-hub -version
curl -s http://HUB:7600/healthz
journalctl -u devhive-hub --no-pager | tail -50
```

---

## 14. الأمان والنسخ الاحتياطي

### مفاتيح يجب حفظها

| الملف | من يملكه | أهميته |
|---|---|---|
| `~/.devhive-tunnel/identity.key` | كل جهاز | هويتك (بصمتك). فقده = هوية جديدة. |
| `~/.devhive-tunnel/config.json` | كل جهاز | الشبكات + كلمات المرور + مفاتيح الإدارة المحفوظة. |
| `~/.devhive-hub/hub.json` | مشغّل الـ Hub | كل حالة الشبكات والدعوات والحظر والتدقيق. |
| `~/.devhive-hub/GM-KEY.txt` | مشغّل الـ Hub | التحكم العام. **لا تشاركه.** |

### نسخ احتياطي

```bash
# جهاز العميل
tar -czf devhive-client-backup.tar.gz -C ~ .devhive-tunnel

# الـ Hub
sudo tar -czf devhive-hub-backup.tar.gz -C /var/lib devhive-hub
```

احفظ النسخ مشفّرة وخارج الجهاز. لا ترفعها إلى مستودعات عامة.

### مبادئ عملية

- شارك **دعوات فردية** بدل كلمة المرور الجماعية.
- راجع الأعضاء بانتظام، وبدّل مفتاح الإدارة (`rotate-admin`) عند مغادرة عضو.
- استخدم HTTPS للوحة عند الوصول من الإنترنت، واحصر لوحة GM على شبكة موثوقة إن أمكن.
- الـ Hub يرى البيانات الوصفية (من متصل، أحجام/توقيت حركة، أسماء شبكات) لكنه لا يفك
  تشفير الدردشة/الملفات/حزم IP.

---

## 15. ملخص الأوامر

```text
devtun setup                     معالج أول مرة
devtun tui                       واجهة كاملة (معالج + دردشة)
devtun ai                        إنشاء شبكة بمساعدة المحرّك الذكي
devtun create --name ...         إنشاء مباشر (أضف --yes لتخطي التأكيد)
devtun join <name|id>            انضمام (--password و--admin-key للدعوة)
devtun list                      عرض الشبكات العامة
devtun status                    هويتك وشبكاتك
devtun chat [net]                دردشة REPL (/dm /send /invite /peers)
devtun listen [net]              استقبال الأحداث والملفات
devtun send <peer> <file> [net]  إرسال ملف
devtun invite                    إنشاء دعوة (للمالك)
devtun lan [net]                 وضع LAN (TUN)
devtun version                   الإصدار
devhive-hub                      تشغيل الخادم
dh-mcp                           خادم MCP (stdio)
```

أهم أعلام الإنشاء:

```text
--hub ADDR      عنوان الـ Hub
--name NAME     اسم الشبكة
--password P    كلمة المرور (تُقترح تلقائيًا إن حُذفت)
--profile P     private|team|community|paranoid
--members N     العدد المتوقع
--lan           تجهيز LAN
--invite        إلزام دعوة
--anonymous     ظهور عام
--nick NAME     اسمك
--yes           بلا تأكيد
```

---

## 16. بناء الإصدارات والتطوير

```bash
make build                 # بناء محلي
make test                  # كل الاختبارات (وحدة + تكامل)
make e2e                   # اختبار شامل: إنشاء→انضمام→دردشة→ملفات→دعوات→GM
make dist                  # حزم لكل المنصات (Linux/Windows/Android)
scripts/termux-bootstrap.sh
```

إن أردت تعديل السلوك:

- `pkg/aigen` — قواعد «الذكاء الاصطناعي» والملفات الجاهزة.
- `pkg/hub` — البروتوكول، التوجيه، REST، لوحة الويب المدمجة في `pkg/hub/web`.
- `pkg/client` — الجلسة، التشفير الثنائي، الملفات، تمرير IP.
- `pkg/tun` / `pkg/mesh` — محوّلات TUN وجسر الشبكة.
- `pkg/tui` — واجهة bubbletea.
- `pkg/mcp` — أدوات وكلاء AI.

بعد التعديل: `make build && make test`.

---

## 17. مسارات الملفات المهمة

| ما | Linux/Termux | Windows |
|---|---|---|
| إعداد العميل | `~/.devhive-tunnel/config.json` | `%APPDATA%\devhive-tunnel\config.json` |
| هوية العميل | `~/.devhive-tunnel/identity.key` | `%APPDATA%\devhive-tunnel\identity.key` |
| الملفات المستلمة | `~/.devhive-tunnel/downloads/` | `%APPDATA%\devhive-tunnel\downloads\` |
| حالة الـ Hub | `~/.devhive-hub/hub.json` | `%USERPROFILE%\.devhive-hub\hub.json` |
| مفتاح GM | `~/.devhive-hub/GM-KEY.txt` | `%USERPROFILE%\.devhive-hub\GM-KEY.txt` |
| لوحة الويب المدمجة | داخل الثنائي (`pkg/hub/web`) | — |

---

## 18. ملاحظات خاصة ببيئتك الحالية

بيئتك: **Termux → Ubuntu PRoot (aarch64)**، Go في `/usr/lib/go/bin/go`، المشروع في
`/root/devhive-tunnel`، **لا يوجد `/dev/net/tun`**، والقرص شبه ممتلئ.

### أوامر جاهزة للنسخ

```bash
export PATH="/usr/lib/go/bin:$PATH"     # أو استخدم المسار الكامل في كل أمر
cd /root/devhive-tunnel

# بناء الثنائيات
make build

# تشغيل Hub محلي للتجربة
bin/devhive-hub -dir /tmp/hive-hub -listen 127.0.0.1:7676 -web 127.0.0.1:7600
```

### قيود يجب تذكّرها

- **LAN غير متاح هنا** (لا يوجد TUN). استخدم الدردشة/الملفات، أو شغّل وضع LAN من
  Windows/Linux حقيقي.
- **القرص ممتلئ تقريبًا** (~300MB متاحة). عند الخطأ `no space left on device`:

  ```bash
  go clean -cache
  rm -rf ./dist        # أعد بناء ما تحتاجه فقط
  TARGETS="windows/amd64 android/arm64" scripts/build-all.sh
  ```

- الثنائيات الجاهزة محليًا موجودة في `bin/`. حزم المنصات في `dist/` تحتاج مساحة أكبر؛
  ابنِ ما تحتاجه فقط:

  ```bash
  TARGETS="windows/amd64 android/arm64" scripts/build-all.sh
  ```

- للتشغيل البعيد من PRoot، تأكد أن المنفذ قابل للوصول (PRoot نفسه لا يغيّر الشبكة، لكن
  شبكة الهاتف/المشغّل قد تحجب المنافذ).

### اختبار سريع (تحققنا منه فعليًا)

```bash
# 1) Hub
bin/devhive-hub -dir /tmp/fin-hub -listen 127.0.0.1:27876 -web 127.0.0.1:27600

# 2) المالك
mkdir -p /tmp/finA
HOME=/tmp/finA bin/devtun create \
  --hub 127.0.0.1:27876 --name final-hive --password 'Fin-Pass-77' \
  --profile private --members 4 --nick alice --yes

# 3) دعوة
HOME=/tmp/finA bin/devtun invite

# 4) عضو جديد
mkdir -p /tmp/finB
HOME=/tmp/finB bin/devtun join \
  --hub 127.0.0.1:27876 --password 'Fin-Pass-77' \
  --admin-key '<الدعوة>' --nick bob final-hive

# 5) دردشة
HOME=/tmp/finB bin/devtun listen --nick bob final-hive          # طرفية مستقبلة
HOME=/tmp/finA bin/devtun chat --nick alice final-hive          # طرفية مرسلة

# 6) ملف
HOME=/tmp/finA bin/devtun send bob ./README.md final-hive

# 7) لوحة
curl -s http://127.0.0.1:27600/healthz
```

---

## 19. الاستضافة المجانية الكاملة (Oracle + DuckDNS + GitHub)

الهدف: أنت تستضيف الخادم مجانًا، والمشتركون يستخدمون التطبيق مجانًا بالكامل، مع مراقبة حيّة للوحة GM وتوزيع احترافي للتطبيق.

### لماذا Oracle Cloud Always Free؟
| الخيار | الحدود المجانية | مناسب؟ |
|---|---|---|
| **Oracle Cloud Always Free** | حتى 4 OCPU ARM و24GB RAM و200GB تخزين، **10TB خروج شهريًا** | ✅ الأفضل (الملفات تمر عبر hub) |
| Google Cloud e2-micro | 1GB خروج شهريًا فقط | ⚠️ للدردشة فقط، لا يصلح للملفات |
| Render/Railway/Fly المجانية | نوم تلقائي، لا تدعم منفذ TCP خام 7676 | ❌ غير مناسبة |
| جهاز منزلي + DuckDNS | كهرباء/إنترنت، وقد تكون خلف CGNAT | بديل احتياطي |

ملاحظات Oracle: بطاقة للتحقق، سعة ARM قد لا تتوفر دائمًا (جرّب لاحقًا أو استخدم amd64)، لا تتجاوز حدود Always Free، وقد تُستعاد الموارد الخاملة، والشروط قد تتغير.

### 1) إنشاء الخادم
1. حساب Oracle Cloud → Always Free → أنشئ Ubuntu 24.04 (يفضل `VM.Standard.A1.Flex` 2–4 OCPU / 12–24GB، أو instanced amd64).
2. أضف مفتاح SSH، ثم افتح في **VCN → Security List** المنافذ: `22` و`80` و`443` و`7676/tcp`.
3. على Ubuntu في Oracle قد تحتاج قواعد iptables أيضًا:
```bash
sudo iptables -I INPUT 6 -p tcp --dport 80 -j ACCEPT
sudo iptables -I INPUT 6 -p tcp --dport 443 -j ACCEPT
sudo iptables -I INPUT 6 -p tcp --dport 7676 -j ACCEPT
sudo netfilter-persistent save
```

### 2) دومين مجاني من DuckDNS
1. ادخل https://www.duckdns.org وسجّل عبر GitHub/Google.
2. أنشئ نطاقًا مثل `myhive` واحفظ الـ token.
3. لا تضبط IP يدويًا: سكربت النشر يحدّثه كل 5 دقائق تلقائيًا.

### 3) إصدار التطبيقات على GitHub (مجاني)
```bash
git tag v0.1.0
git push origin v0.1.0
```
- `.github/workflows/release.yml` يبني تلقائيًا: `devtun` و`devhive-hub` و`dh-mcp` لكل من linux-amd64 وlinux-arm64 وwindows-amd64 وandroid-arm64، مع `checksums.txt`.
- عدّاد التنزيلات لكل ملف يظهر مجانًا في صفحة **Releases → Assets** (هذه مراقبة التنزيلات دون أي خادم إضافي).
- المستخدمون يحمّلون مباشرة من GitHub، فلا يستهلك hub ترافيك التنزيل.

### 4) النشر بأمر واحد
على الخادم (Ubuntu، بصلاحية root):
```bash
sudo -i
DOMAIN=myhive.duckdns.org \
EMAIL=you@example.com \
REPO=you/devhive-tunnel \
DUCKDNS_DOMAIN=myhive \
DUCKDNS_TOKEN=xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx \
bash scripts/deploy-vps.sh
```
- يثبّت: `devhive-hub` + Caddy (HTTPS تلقائي) + systemd + ufw + محدّث DuckDNS.
- بديل دون GitHub: `BINARY=./devhive-hub-linux-arm64 ... bash scripts/deploy-vps.sh`.
- المخرجات: `https://myhive.duckdns.org` ومسار مفتاح GM `/var/lib/devhive-hub/GM-KEY.txt`.

### 5) ماذا يفعل المشتركون؟ (مجانًا تمامًا)
```bash
devtun join --hub myhive.duckdns.org:7676 --password 'كلمة-الشبكة' --nick اسمك
```
- لا حساب، لا اشتراك، لا دفع: فقط اسم الشبكة وكلمتها.
- حمّل Windows/Linux/Termux من صفحة Releases نفسها.

### 6) المراقبة الحيّة وتطبيق PWA
- لوحة GM تعرض الآن بيانات حيّة عبر SSE كل ثانية: الشبكات، المتصلون، المحاولات، الحظر، وسجل التدقيق (بيانات وصفية فقط؛ المحادثات والملفات تبقى مشفّرة طرفًا لطرف).
- افتح اللوحة على الهاتف ثم **إضافة إلى الشاشة الرئيسية** لتثبيتها كتطبيق PWA (تعمل واجهتها دون اتصال، والبيانات تحتاج اتصالًا).
- للمراقبة: راقب `journalctl -u devhive-hub -f` وعدّاد التنزيلات في GitHub.

### 7) نسخ احتياطي وتحديث
```bash
sudo tar czf /root/devhive-backup-$(date +%F).tgz /var/lib/devhive-hub
```
- أضفه إلى cron أسبوعيًا واحفظ النسخة خارج الخادم.
- للتحديث: أعد تشغيل `deploy-vps.sh` بعد إصدار جديد؛ البيانات تبقى في `/var/lib/devhive-hub`.

### التكلفة
**0$** ضمن حدود Always Free: الخادم + DuckDNS + GitHub Releases + GitHub Actions. راقب حدود الحساب، وفعّل تنبيهات الفوترة.

---

### أخيرًا

- ابدأ دائمًا بـ **Hub** يعمل، ثم **إنشاء شبكة**، ثم **دعوة**، ثم **انضمام**.
- احفظ مفاتيحك قبل أي شيء آخر.
- عند أي رسالة خطأ، راجع [جدول استكشاف الأخطاء](#13-استكشاف-الأخطاء).

مرحبًا بك في الخليّة 🐝
