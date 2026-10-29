[English](../en/runner/install.md)

# تثبيت جهاز تشغيل

جهاز التشغيل (runner) ملف تنفيذي واحد ثابت لـ Linux، اسمه `dhow-runner`، لمعماريتي `amd64` و`arm64`. كل إصدار يأتي به في أرشيف موقّع مع خدمة systemd الخاصة به. الأمر `get.sh --component runner` ينزّل هذا الأرشيف ويفحصه ويثبّت الملفين. أما الخطوات التي بعد ذلك (المستخدم، والتسجيل، والتشغيل) فما زالت يدوية كما يلي، حتى يأتي `dhow-runner install` في إصدار لاحق.

## المتطلبات

- **Linux** على `amd64` أو `arm64`. لا ندعم أجهزة تشغيل Windows وmacOS.
- **اتصال HTTPS خارجي** إلى سيرفر داو (Dhow)، مباشرة أو عبر وكيل HTTP (نحترم `HTTPS_PROXY`، ويبقى TLS من الطرف إلى الطرف عبره).
- **مستخدم مخصّص**، `dhow-runner`، لا يملك أي شيء آخر. جهاز التشغيل لا يحتاج إلى root أبدًا.
- **لمشغّل Podman (M3.6):** Podman 5.4 أو أحدث، و`crun`، و`uidmap`، وcgroup v2 مع التفويض (delegation)، وuser namespaces. هذه موجودة في Debian 13 وUbuntu 26.04 وFedora وRHEL 9.6+ (Rocky وAlma). Debian 12 وUbuntu 24.04 لا يوفّرانها من مستودعاتهما. يمنع Ubuntu 24.04 أيضًا user namespaces للبرامج التي ليس لها ملف AppArmor. اضبط `kernel.apparmor_restrict_unprivileged_userns=0` في `/etc/sysctl.d/` أو أعطِ Podman ملفًا، وإلا تفشل فحوص البدء في جهاز التشغيل.
- **Sub-ids لمستخدم جهاز التشغيل:** 65536 على الأقل لكل تشغيل يعمل في الوقت نفسه، في `/etc/subuid` و`/etc/subgid` (كل تشغيل يحصل على user namespace خاص به).
- **جهاز خاص به، أو `/proc` خاص.** لا يستطيع المستخدمون المحليون الآخرون عرض مجلد حالة جهاز التشغيل. لكن معرّف الحاوية يظهر في سطر أوامر `conmon`، وبه قد يقرأ مستخدم محلي ملفات صورة التشغيل (وليس `/workspace` أو الأسرار أبدًا، فهي في tmpfs الخاص بالحاوية). شغّل جهاز التشغيل على جهاز لا يدخل إليه أحد غيرك، أو اربط (mount) `/proc` بالخيار `hidepid=2`.


## احصل على الملف التنفيذي

### عبر get.sh

نزّله، واقرأه، ثم شغّله. لا تمرّره أبدًا إلى shell مباشرة:

```sh
curl -fsSLO https://raw.githubusercontent.com/Khaymah/dhow/main/deploy/install/get.sh
less get.sh
bash get.sh --component runner
```

الخيار `--version 1.2.3` يختار إصدارًا، والافتراضي هو آخر إصدار. يفعل السكربت ما يلي:

1. ينزّل `dhow-runner_<version>_linux_<arch>.tar.gz` و`SHA256SUMS` و`SHA256SUMS.sigstore.json` من إصدار GitHub؛
2. يفحص توقيع cosign على `SHA256SUMS` مقابل workflow الإصدار في `Khaymah/dhow` (بلا مفتاح، عبر Sigstore) إذا كان `cosign` مثبّتًا، ويحذّر بأنه يفحص الـ checksum فقط إذا لم يكن مثبّتًا؛
3. يفحص الأرشيف مقابل سطره في `SHA256SUMS`، ويتوقف إذا كان السطر ناقصًا أو لا يطابق؛
4. يثبّت `/usr/local/bin/dhow-runner` (`0755`) و`/etc/systemd/system/dhow-runner.service` (`0644`)، ويعيد تحميل systemd. لا يفعّل الخدمة ولا يشغّلها.

لا يشغّل `dhow-runner install` بعد، بل يطبع الخطوات التالية بدلًا من ذلك. ثبّت `cosign` أولًا لتحصل على فحص التوقيع: بدونه، يثبت الـ checksum فقط أن التنزيل لم يتلف، ولا يثبت من بناه. مع مرآة (`DHOW_DOWNLOAD_URL`) يصبح cosign إلزاميًا، كما في [دليل المثبّت](../install/installer.md). عند تشغيله بمستخدم عادي، ينسخ السكربت الملفات إلى مجلد لا يكتب فيه إلا root، ويفحصها ويفك أرشيفها هناك قبل أن يثبّت أي شيء بـ `sudo`.

### يدويًا

نزّل الملفات الثلاثة نفسها من صفحة الإصدار، ثم:

```sh
cosign verify-blob --bundle SHA256SUMS.sigstore.json \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  --certificate-identity-regexp '^https://github\.com/Khaymah/dhow/\.github/workflows/release\.yml@refs/tags/v' \
  SHA256SUMS
sha256sum --check --ignore-missing SHA256SUMS
tar -xzf dhow-runner_*_linux_amd64.tar.gz
sudo install -m 0755 dhow-runner /usr/local/bin/dhow-runner
sudo install -m 0644 dhow-runner.service /etc/systemd/system/dhow-runner.service
dhow-runner version
```

لكل أرشيف أيضًا SPDX SBOM (`<archive>.spdx.json`) وإثبات بناء SLSA على صفحة الإصدار.

### من الكود المصدري

بإصدار Go المثبّت في `go.mod`:

```sh
make build-runner
sudo install -m 0755 bin/dhow-runner /usr/local/bin/dhow-runner
sudo install -m 0644 deploy/systemd/dhow-runner.service /etc/systemd/system/dhow-runner.service
```

## أنشئ المستخدم والمجلدات

```sh
sudo useradd --system --home-dir /var/lib/dhow-runner --shell /usr/sbin/nologin dhow-runner
sudo usermod --add-subuids 1000000-2048575 --add-subgids 1000000-2048575 dhow-runner
sudo install -d -m 0700 -o dhow-runner -g dhow-runner /var/lib/dhow-runner
sudo install -d -m 0750 -o root -g dhow-runner /etc/dhow-runner
sudo install -m 0640 -o root -g dhow-runner /dev/null /etc/dhow-runner/runner.env
```

- نطاق الـ sub-ids يعطي جهاز التشغيل 1048576 معرّفًا، وهذا يكفي 16 تشغيلًا في الوقت نفسه (65536 لكل تشغيل). اختر نطاقًا لا يتداخل معه أي سطر آخر في `/etc/subuid` و`/etc/subgid`.
- `/var/lib/dhow-runner` هو مجلد الحالة. فيه الهوية (المفاتيح الخاصة)، وسجل الحجوزات (journal)، ومجلد لكل محاولة تعمل. يرفض جهاز التشغيل البدء إذا استطاع مستخدمون آخرون الكتابة فيه، أو إذا كان ملف `identity` أو `journal` مقروءًا لأي أحد غير مستخدم جهاز التشغيل.
- `/etc/dhow-runner/runner.env` فيه [الإعدادات](configuration.md). يمكن أن يبقى فارغًا. ليس فيه أسرار، لكن اجعل الكتابة فيه لـ root فقط. ترفض الخدمة البدء بدونه.

## خدمة systemd

يأتي الإصدار بـ [`deploy/systemd/dhow-runner.service`](../../deploy/systemd/dhow-runner.service). تشغّل `dhow-runner run` بالمستخدم `dhow-runner`، وتقرأ `/etc/dhow-runner/runner.env`، وتعيد التشغيل عند الفشل، وتعطي جهاز التشغيل 60 ثانية ليتوقف. لا تعدّل الملف الذي يأتي مع الإصدار. غيّره بملف إضافي (drop-in) عبر `sudo systemctl edit dhow-runner`، مثلًا لتضيف `ReadWritePaths=` عندما لا يكون `DHOW_RUNNER_STATE_DIR` هو `/var/lib/dhow-runner`.

ما تضبطه الخدمة، ولماذا:

- `Delegate=yes` يسلّم شجرة cgroup الفرعية لجهاز التشغيل، حتى يضبط مشغّل Podman حدودًا لكل تشغيل. و`KillMode=control-group` يقتل كل عمليات التشغيلات مع جهاز التشغيل.
- `MemorySwapMax=0` يبعد ذاكرة التشغيلات، والـ tmpfs الذي يحفظ ملفات الأسرار، عن الـ swap. و`LimitCORE=0` يبعد المفاتيح عن core dumps.
- `StateDirectory=dhow-runner` مع `StateDirectoryMode=0700`: القيمة الافتراضية في systemd، `0755`، ستجعل جهاز التشغيل يرفض البدء.
- عند `systemctl stop` يتوقف جهاز التشغيل عن أخذ العمل، ويقتل البيئات المعزولة التي تعمل، ويبلغ عنها كمفقودة خلال 15 ثانية، حتى يعيد السيرفر محاولتها في مكان آخر.

### التحصين

يقيّم `systemd-analyze security dhow-runner.service` الخدمة بـ **5.2 MEDIUM** (systemd 255 و259)، ويفشل CI إذا تجاوز التقييم 6.9. المستوى أقل من مستوى السيرفر لأن جهاز التشغيل يدير حاويات: Podman بلا root يحتاج إلى `newuidmap`، وهو أداة setuid (أو بـ file capabilities)، وإلى كل أنواع الـ namespaces. جرّبنا كل توجيه أدناه مع Podman بلا root يشغّل صورة تشغيل داخل الخدمة، ويشغّل CI مجموعتي اختبار الهروب والحدود داخل العزل نفسه.

| التوجيه | الأثر |
|---|---|
| `ProtectSystem=strict`، `ReadWritePaths=/var/lib/dhow-runner` | نظام الملفات كله للقراءة فقط إلا مجلد الحالة. |
| `InaccessiblePaths=` `sudo`، `su`، `pkexec`، `passwd`، `chsh`، `chfn`، `newgrp`، `gpasswd`، `mount`، `umount` | نخفي برامج setuid التي لا يحتاجها جهاز التشغيل أبدًا، فلا يستطيع أي منها استخدام الـ bounding set. أدوات Podman نفسها، `newuidmap` و`newgidmap`، تبقى. ولا يحتاج crun ولا مشغّل overlay الأصلي إلى `mount` أو `fusermount3`. |
| `ProtectHome=yes`، `PrivateTmp=yes` | لا مجلدات منزلية، و`/tmp` و`/var/tmp` خاصان. |
| `ProtectProc=invisible` | يرى جهاز التشغيل عملياته وعمليات تشغيلاته فقط (يملك user namespaces الخاصة بها)، ولا يرى عمليات المستخدمين الآخرين. |
| `CapabilityBoundingSet=CAP_SETUID CAP_SETGID CAP_DAC_OVERRIDE CAP_SYS_ADMIN CAP_SYS_PTRACE` | جهاز التشغيل نفسه بلا capabilities. الـ bounding set يحدد فقط ما يستطيع برنامج setuid يشغّله أن يكسبه. هذه الخمسة هي ما يحتاجه `newuidmap` بخاصية setuid في Debian وUbuntu. جرّبنا حذف أي واحد من `CAP_SYS_ADMIN` أو `CAP_DAC_OVERRIDE` أو `CAP_SYS_PTRACE`، فلم يتمكن Podman من تجهيز user namespace للتشغيل. `CAP_SETUID` و`CAP_SETGID` يكتبان خرائط المعرّفات، و`CAP_DAC_OVERRIDE` يفتح `uid_map` للعملية الجديدة كـ root، و`CAP_SYS_ADMIN` تفحصه النواة عند كتابة خريطة لعملية أخرى، و`CAP_SYS_PTRACE` يسمح له برؤية تلك العملية بينما `/proc` يخفي المستخدمين الآخرين (أدناه، أو `hidepid=` على الجهاز). في Fedora وRHEL، حيث لـ `newuidmap` file capabilities بدل setuid، يُستخدم الأولان فقط. كل capability أخرى، مثل `CAP_SYS_MODULE` أو `CAP_NET_ADMIN` أو `CAP_SYS_RAWIO`، غير موجودة. user namespace الخاص بالتشغيل يبدأ بمجموعة جديدة، فلا تتأثر التشغيلات. |
| `AmbientCapabilities=` | لا capabilities موروثة. |
| `PrivateIPC=yes` | System V IPC وطوابير رسائل POSIX خاصة به. |
| `ProtectKernelModules=yes`، `ProtectClock=yes` | لا تحميل لوحدات النواة، ولا تغيير للساعة. |
| `RestrictRealtime=yes`، `LockPersonality=yes` | لا جدولة realtime، ولا تغيير للـ personality. |
| `RestrictAddressFamilies=AF_UNIX AF_INET AF_INET6 AF_NETLINK` | أنواع الـ sockets نفسها التي تسمح بها البيئة المعزولة للتشغيل. |
| `SystemCallFilter=~@clock @module @obsolete @raw-io @swap`، `SystemCallErrorNumber=EPERM` | الـ system calls التي تحتاج إلى capabilities في الجهاز لا تملكها التشغيلات أبدًا، أو القديمة، ترجع `EPERM` لجهاز التشغيل ولكل ما يشغّله. |
| `RemoveIPC=yes`، `KeyringMode=private` | لا كائنات IPC متروكة بعده، وkeyring خاص في النواة. |

توجيهات لم نضعها، ولكل منها سبب:

| التوجيه | لماذا لا نضبطه |
|---|---|
| `NoNewPrivileges=yes` | `newuidmap` و`newgidmap` يكسبان صلاحياتهما عبر setuid أو file capabilities، و`no_new_privs` يمنع ذلك، فلا يستطيع Podman ربط الـ sub-ids الخاصة بالتشغيل. وما زالت التشغيلات تعمل مع `no-new-privileges` داخل بيئتها المعزولة. |
| `PrivateUsers=yes` | user namespace الخاص بالخدمة سيربط مستخدم جهاز التشغيل فقط، فلا يمكن ربط الـ 65536 sub-id الخاصة بالتشغيل. |
| `RestrictNamespaces=` | ينشئ Podman لكل تشغيل namespaces من نوع user وmount وPID وnetwork وUTS وIPC وcgroup. |
| `ProtectKernelTunables=yes`، `ProtectKernelLogs=yes`، `ProtectHostname=yes`، `ProcSubset=pid` | تغطي أجزاء من `/proc`، فترفض النواة ربط `/proc` الجديد الذي ينشئه crun داخل التشغيل (`mount proc: Operation not permitted`). و`ProcSubset=pid` يخفي أيضًا `/proc/sys/kernel/cap_last_cap`، الذي يقرؤه Podman. |
| `ProtectControlGroups=yes` | يجعل `/sys/fs/cgroup` للقراءة فقط، وجهاز التشغيل يكتب حدود كل تشغيل في الشجرة المفوّضة له. |
| `PrivateDevices=yes`، `DevicePolicy=closed` | `/dev` الخاص وقائمة الأجهزة المغلقة ينطبقان على كل حاوية تشغيل أيضًا، وPodman بلا root يفشل في بدء حاوية مع أي منهما. |
| `MemoryDenyWriteExecute=yes` | مرشحات seccomp تُورَّث، فستفقد كل التشغيلات الذاكرة القابلة للكتابة والتنفيذ: Node.js وJVM و.NET وكل بيئات JIT الأخرى ستنهار. |
| `SystemCallArchitectures=native` | تورثه التشغيلات، فيمنع ملفات x86 وARM بـ 32 بت التي تسمح بها البيئة المعزولة. |
| `SystemCallFilter=@system-service` ومجموعات المنع `@mount` و`@privileged` و`@debug` و`@cpu-emulation` و`@reboot` و`@resources` و`@keyring` | يحتاج Podman وcrun إلى استدعاءات mount وnamespace وsetuid وkeyring، والباقي سيأخذ استدعاءات مسموحة للتشغيلات اليوم (`modify_ldt` و`pidfd_getfd` و`sched_setaffinity` و`reboot` داخل PID namespace الخاص بالتشغيل). |
| `RestrictSUIDSGID=yes` | يرثه Podman عند فك الصور: الصورة التي فيها أي ملف setuid (أغلب صور التوزيعات فيها `su` أو `passwd`) تفشل في السحب. |
| `UMask=0077` | الفائدة قليلة: جهاز التشغيل ينشئ ملفاته بـ `0600` ومجلداته بـ `0700` بالفعل. ويجب أن يمر user namespace الخاص بالتشغيل عبر بعض المجلدات التي ينشئها Podman، ولم نختبر المشغّل كاملًا بقناع أضيق. |
| `PrivateNetwork=yes`، `IPAddressDeny=` | يجب أن يصل جهاز التشغيل إلى السيرفر، وإلى سجلات الصور للتشغيلات. |

الخطوة التالية: [سجّل جهاز التشغيل](register.md).
