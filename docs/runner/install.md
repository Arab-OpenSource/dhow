[English](../en/runner/install.md)

# تثبيت جهاز تشغيل

جهاز التشغيل (runner) ملف تنفيذي واحد ثابت لـ Linux، اسمه `dhow-runner`، لمعماريتي `amd64` و`arm64`. التثبيت كحزمة عبر الوضع `runner` في المثبّت التفاعلي يأتي مع M3.10. حتى ذلك الوقت، جهّزه يدويًا كما يلي.

## المتطلبات

- **Linux** على `amd64` أو `arm64`. لا ندعم أجهزة تشغيل Windows وmacOS.
- **اتصال HTTPS خارجي** إلى سيرفر داو (Dhow)، مباشرة أو عبر وكيل HTTP (نحترم `HTTPS_PROXY`، ويبقى TLS من الطرف إلى الطرف عبره).
- **مستخدم مخصّص**، `dhow-runner`، لا يملك أي شيء آخر. جهاز التشغيل لا يحتاج إلى root أبدًا.
- **لمشغّل Podman (M3.6):** Podman 5.4 أو أحدث، و`crun`، و`uidmap`، وcgroup v2 مع التفويض (delegation)، وuser namespaces. هذه موجودة في Debian 13 وUbuntu 26.04 وFedora وRHEL 9.6+ (Rocky وAlma). Debian 12 وUbuntu 24.04 لا يوفّرانها من مستودعاتهما. يمنع Ubuntu 24.04 أيضًا user namespaces للبرامج التي ليس لها ملف AppArmor. اضبط `kernel.apparmor_restrict_unprivileged_userns=0` في `/etc/sysctl.d/` أو أعطِ Podman ملفًا، وإلا تفشل فحوص البدء في جهاز التشغيل.
- **Sub-ids لمستخدم جهاز التشغيل:** 65536 على الأقل لكل تشغيل يعمل في الوقت نفسه، في `/etc/subuid` و`/etc/subgid` (كل تشغيل يحصل على user namespace خاص به).
- **جهاز خاص به، أو `/proc` خاص.** لا يستطيع المستخدمون المحليون الآخرون عرض مجلد حالة جهاز التشغيل. لكن معرّف الحاوية يظهر في سطر أوامر `conmon`، وبه قد يقرأ مستخدم محلي ملفات صورة التشغيل (وليس `/workspace` أو الأسرار أبدًا، فهي في tmpfs الخاص بالحاوية). شغّل جهاز التشغيل على جهاز لا يدخل إليه أحد غيرك، أو اربط (mount) `/proc` بالخيار `hidepid=2`.

## احصل على الملف التنفيذي

ابنِه من نسخة من المستودع، بإصدار Go المثبّت في `go.mod`:

```sh
make build-runner
sudo install -m 0755 bin/dhow-runner /usr/local/bin/dhow-runner
dhow-runner version
```

## أنشئ المستخدم والمجلدات

```sh
sudo useradd --system --home-dir /var/lib/dhow-runner --shell /usr/sbin/nologin dhow-runner
sudo install -d -m 0700 -o dhow-runner -g dhow-runner /var/lib/dhow-runner
sudo install -d -m 0750 -o root -g dhow-runner /etc/dhow-runner
```

- `/var/lib/dhow-runner` هو مجلد الحالة. فيه الهوية (المفاتيح الخاصة)، وسجل الحجوزات (journal)، ومجلد لكل محاولة تعمل. يرفض جهاز التشغيل البدء إذا استطاع مستخدمون آخرون الكتابة فيه، أو إذا كان ملف `identity` أو `journal` مقروءًا لأي أحد غير مستخدم جهاز التشغيل.
- `/etc/dhow-runner/runner.env` فيه [الإعدادات](configuration.md). ليس فيه أسرار، لكن اجعل الكتابة فيه لـ root فقط.

## خدمة systemd

```ini
[Unit]
Description=Dhow runner
Wants=network-online.target
After=network-online.target

[Service]
User=dhow-runner
Group=dhow-runner
EnvironmentFile=/etc/dhow-runner/runner.env
ExecStart=/usr/local/bin/dhow-runner run
Restart=on-failure
RestartSec=10s
StateDirectory=dhow-runner
StateDirectoryMode=0700
Delegate=yes
MemorySwapMax=0
LimitCORE=0
TimeoutStopSec=60s

[Install]
WantedBy=multi-user.target
```

- `StateDirectoryMode=0700` مهم: القيمة الافتراضية في systemd، `0755`، تجعل جهاز التشغيل يرفض البدء.
- `MemorySwapMax=0` يبعد ذاكرة التشغيلات، والـ tmpfs الذي يحفظ ملفات الأسرار، عن الـ swap. و`LimitCORE=0` يبعد المفاتيح عن core dumps.
- `Delegate=yes` يسلّم شجرة cgroup الفرعية لجهاز التشغيل، حتى يضبط مشغّل Podman حدودًا لكل تشغيل.
- عند `systemctl stop` يتوقف جهاز التشغيل عن أخذ العمل، ويقتل البيئات المعزولة التي تعمل، ويبلغ عنها كمفقودة خلال 15 ثانية، حتى يعيد السيرفر محاولتها في مكان آخر.

الخطوة التالية: [سجّل جهاز التشغيل](register.md).
