[English](../en/runner/install.md)

# تثبيت جهاز تشغيل

جهاز التشغيل (runner) ملف تنفيذي واحد ثابت لـ Linux، اسمه `dhow-runner`، لمعماريتي `amd64` و`arm64`. كل إصدار يأتي به في أرشيف موقّع مع خدمة systemd الخاصة به. الأمر `get.sh --component runner` ينزّل هذا الأرشيف ويفحصه ويثبّت الملفين. ثم يكمل `sudo dhow-runner install` الباقي: يفحص الجهاز، وينشئ مستخدم جهاز التشغيل والـ sub-ids والمجلدات، ويسجّل جهاز التشغيل، ويحفظ هويته مشفّرة بـ `systemd-creds` إذا سمح الجهاز بذلك، ثم يشغّله.

## المتطلبات

- **Linux** على `amd64` أو `arm64`، مع systemd. لا ندعم أجهزة تشغيل Windows وmacOS.
- **اتصال HTTPS خارجي** إلى سيرفر داو (Dhow)، مباشرة أو عبر وكيل HTTP (نحترم `HTTPS_PROXY`، ويبقى TLS من الطرف إلى الطرف عبره).
- **رمز تسجيل** من صفحة **أجهزة التشغيل** في السيرفر ([تسجيل جهاز تشغيل](register.md#1-إنشاء-رمز-تسجيل)).
- **لمشغّل Podman (M3.6):** Podman 5.4 أو أحدث، و`crun`، و`uidmap`، وcgroup v2 مع التفويض (delegation)، وuser namespaces. هذه موجودة في Debian 13 وUbuntu 26.04 وFedora وRHEL 9.6+ أو 10 (Rocky وAlma). Debian 12 وUbuntu 24.04 لا يوفّرانها من مستودعاتهما، ويتوقف `install` عليهما ويذكر الإصدار الذي وجده. ويمنع Ubuntu أيضًا user namespaces للبرامج التي ليس لها ملف AppArmor. اضبط `kernel.apparmor_restrict_unprivileged_userns=0` في `/etc/sysctl.d/` أو أعطِ Podman ملفًا، وإلا يتوقف `install` ويخبرك بالسبب.
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

في النهاية يطلب منك تشغيل `sudo dhow-runner install`. ثبّت `cosign` أولًا لتحصل على فحص التوقيع: بدونه، يثبت الـ checksum فقط أن التنزيل لم يتلف، ولا يثبت من بناه. مع مرآة (`DHOW_DOWNLOAD_URL`) يصبح cosign إلزاميًا، كما في [دليل المثبّت](../install/installer.md). عند تشغيله بمستخدم عادي، ينسخ السكربت الملفات إلى مجلد لا يكتب فيه إلا root، ويفحصها ويفك أرشيفها هناك قبل أن يثبّت أي شيء بـ `sudo`.

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

## التثبيت بالأمر dhow-runner install

```sh
sudo dhow-runner install
```

يعرض ما وجده على الجهاز، ثم يسأل عن عنوان السيرفر، ورمز التسجيل (لا يظهر أثناء الكتابة)، واسم جهاز التشغيل، ومكان حفظ المفاتيح الخاصة، وعدد التشغيلات التي تعمل في الوقت نفسه، وعلى الجهاز الصغير يسأل عن الذاكرة والقرص لكل تشغيل. ثم يطبع الخطة كاملة، ويسأل مرة واحدة قبل أن يغيّر أي شيء. الخيار `--dry-run` يتوقف بعد الخطة، و`--yes` يتخطى السؤال الأخير، و`--lang ar` أو `--lang en` يختار اللغة (الافتراضي يتبع إعدادات اللغة في النظام).

### ما الذي يفعله

كل خطوة تفحص أولًا هل تمت من قبل، لذلك لا يغيّر تشغيل `install` مرة أخرى إلا الناقص:

1. **يفحص** Podman 5.4+، و`crun`، و`newuidmap`، وcgroup v2، وuser namespaces، وإعداد AppArmor، و[الذاكرة لكل تشغيل](#الذاكرة-لكل-تشغيل)، ويتوقف بسبب واضح إذا نقص شيء. ومع `install_packages: true` يثبّت `podman` و`crun` و`uidmap` (أو `shadow-utils`) من التوزيعة أولًا، ثم يفحص إصدار Podman مرة أخرى.
2. **ينشئ مستخدم النظام** `dhow-runner`، دون shell للدخول، ومجلده المنزلي `/var/lib/dhow-runner`.
3. **يعطيه 1048576 من الـ sub-ids** (16 تشغيلًا في الوقت نفسه، 65536 لكل تشغيل) في `/etc/subuid` و`/etc/subgid`، من أول نطاق فارغ يبدأ من 1048576 أو بعده.
4. **ينشئ** `/var/lib/dhow-runner` (`0700 dhow-runner`، ومشغّل Podman يجعلها `0711` عندما يبدأ) و`/etc/dhow-runner` (`0750 root:dhow-runner`).
5. **يكتب** `/etc/dhow-runner/runner.env` (`0640 root:dhow-runner`) بعنوان السيرفر، والـ origin، وملف CA، والمشغّل، والسعة، وأي [إعدادات](configuration.md) أخرى. الأسطر التي أضفتها بنفسك ولا تديرها الإجابات تبقى كما هي.
6. **يثبّت** الملف التنفيذي في `/usr/local/bin/dhow-runner` والخدمة في `/etc/systemd/system/dhow-runner.service`.
7. **يسجّل** جهاز التشغيل برمز التسجيل. يقرأ الرمز من الملف، أو من الإدخال القياسي، أو من السؤال المخفي، ويحفظه في الذاكرة فقط: لا يضعه أبدًا في سطر أوامر، ولا في متغيرات البيئة، ولا في سجل، ولا على القرص.
8. **يحفظ الهوية مشفّرة** بـ `systemd-creds` في `/var/lib/dhow-runner/identity.cred`، ويضيف ملف drop-in هو `/etc/systemd/system/dhow-runner.service.d/10-dhow-identity.conf` الذي يمرّرها إلى الخدمة ([الهوية المشفّرة](keys.md#الهوية-المشفرة)). إذا لم يستطع `systemd-creds` التشفير على الجهاز (لا توجد شريحة TPM2 ولا مفتاح جهاز صالح)، أو اخترت `identity: file`، تبقى الهوية في `/var/lib/dhow-runner/identity` بالصلاحية `0600`، وتذكر الخطة والنتيجة ذلك. والهوية غير المشفّرة التي تركها إعداد يدوي يشفّرها في مكانها.
9. **يحفظ الإجابات**، دون رمز التسجيل، في `/etc/dhow-runner/answers.yaml` (`0600 root`).
10. **يفعّل الخدمة ويشغّلها**، ثم ينتظر حتى دقيقتين حتى يسجّل جهاز التشغيل الدخول ويطلب عملًا. إذا لم يحدث ذلك، يطبع السبب وآخر أسطر سجل الخدمة.

### الذاكرة لكل تشغيل

يأخذ كل تشغيل ذاكرة حتى `DHOW_RUNNER_MAX_MEMORY`، وقرص tmpfs لمجلد `/workspace` حتى `DHOW_RUNNER_MAX_DISK`، وصفحات tmpfs تُحسب من الذاكرة أيضًا. القيم الافتراضية (2 GiB مع 4 GiB) تحتاج إلى جهاز فيه 6 GiB ذاكرة، ويرفض جهاز التشغيل البدء إذا كان المجموع أكبر مما في الجهاز ([الحدود والجهاز](configuration.md#الحدود-والجهاز)). لذلك يقرأ `install` ذاكرة الجهاز من `/proc/meminfo` قبل الخطوة الأولى، ويطبّق القاعدة نفسها:

- إذا كانت القيم الافتراضية مناسبة، لا يتغير شيء.
- إذا لم تكن مناسبة ولم تكتب أيًّا من القيمتين، تختار الخطة حدودًا أقل. تترك 1 GiB أو ربع الذاكرة، أيهما أكبر، لجهاز التشغيل والنظام. ثم تقسم الباقي بين الذاكرة والقرص بنسبة 1 إلى 2 مثل القيم الافتراضية، بخطوات من 64 MiB، ولا تنزل عن 512 MiB لأي منهما. وتكتب القيمتين في `runner.env` وفي الإجابات المحفوظة، وتذكر ذلك في الخطة. على VPS بحجم 4 GiB يُظهر 3922 MiB، تصبح القيم `DHOW_RUNNER_MAX_MEMORY=960MiB` و`DHOW_RUNNER_MAX_DISK=1920MiB`. والتثبيت بالأسئلة يعرض هذه القيم كإجابة افتراضية، و`--answers` يستخدمها دون سؤال.
- إذا كتبت إحدى القيمتين بنفسك (في `env` أو في `runner.env`) وكان المجموع أكبر من الجهاز، أو كان في الجهاز أقل من 2 GiB، وهذا لا يكفي حتى لأقل الحدود، يتوقف `install` قبل الخطوة الأولى، ولا يغيّر شيئًا، ويذكر القيم التي تكتبها.

ويجري `dhow-runner doctor` الفحص نفسه بالرسالة نفسها.

### دون أسئلة: ملف الإجابات

الخيار `--answers <file>` يقرأ كل الإجابات من ملف YAML ولا يسأل عن شيء:

```yaml
url: https://dhow.example.com
token_file: /root/dhow-runner-token
name: build-01
identity: auto
capacity: 2
install_packages: true
env:
  DHOW_RUNNER_MAX_MEMORY: 4GiB
```

```sh
sudo dhow-runner install --answers runner.yaml
```

| المفتاح | الافتراضي | المعنى |
|---|---|---|
| `url` | (مطلوب) | عنوان السيرفر، كما في `DHOW_RUNNER_URL`. |
| `token_file` | (مطلوب في أول تثبيت) | ملف فيه رمز التسجيل، أو `-` لقراءته من الإدخال القياسي. لا تحتاجه بعد تسجيل جهاز التشغيل، ولا نحفظه أبدًا. |
| `name` | اسم الجهاز | الاسم الذي يظهر في داو. |
| `server_origin` | الـ origin الخاص بـ `url` | `DHOW_RUNNER_SERVER_ORIGIN`، لـ listener منفصل لأجهزة التشغيل. |
| `ca_file` | شهادات النظام | `DHOW_RUNNER_CA_FILE`. |
| `driver` | `podman` | مشغّل العزل. نسخ الاختبار تقبل `fake` أيضًا. |
| `capacity` | `1` | `DHOW_RUNNER_CAPACITY`، من 1 إلى 16. |
| `identity` | `auto` | `encrypted` (يحتاج `systemd-creds`)، أو `file`، أو `auto` الذي يشفّر إذا سمح الجهاز. |
| `install_packages` | `false` | تثبيت الـ packages الناقصة من التوزيعة. |
| `env` | لا شيء | إعدادات `DHOW_RUNNER_*` أخرى لملف `runner.env`. |
| `dev` | `false` | يسمح بـ `http://` العادي إلى عنوان loopback، للتطوير المحلي فقط. |
| `lang` | لغة النظام | `en` أو `ar`. |

نرفض المفاتيح غير المعروفة، ووثيقة YAML ثانية، والملف الأكبر من 64 KiB. ونرفض أيضًا `--token` في سطر الأوامر. وبصلاحيات root يجب أن يكون ملف الإجابات ملكًا لـ root، ولا يكتب فيه إلا root، وفي مجلد لا يكتب فيه الآخرون (أو مجلد sticky مثل `/tmp`). ولا يجوز أن يكون هو أو ملف رمز التسجيل رابطًا رمزيًا (symlink)، ونرفض ملف الرمز إذا استطاع الآخرون الكتابة فيه، ونحذّرك إذا استطاعوا قراءته. والقيم التي نكتبها في `runner.env` لا تقبل رموز التحكم ولا علامات التنصيص ولا الشرطة المائلة العكسية، و`env` لا يقبل إلا الإعدادات المذكورة في [إعدادات جهاز التشغيل](configuration.md).

## افحص جهاز التشغيل

```sh
sudo dhow-runner doctor
```

يفحص متطلبات العزل، والخدمة وتحصينها، والملفات وصلاحياتها، ومكان حفظ الهوية، وتغيير مفاتيح لم يكتمل، وهل يرد السيرفر وهل يسجّل جهاز التشغيل الدخول. يخرج برمز غير صفري إذا وجد مشكلة، والخيار `--fix` يطبّق الإصلاحات التي يقترحها. اقرأ [فحص جهاز التشغيل بالأمر doctor](doctor.md).

## أزل جهاز التشغيل

```sh
sudo dhow-runner uninstall
```

يوقف الخدمة ويعطّلها، ثم يلغي جهاز التشغيل نهائيًا على السيرفر: يوقّع جهاز التشغيل طلب الإلغاء بمفتاحه، وينهي السيرفر جلساته ويسجّل `runner.revoked`. إذا لم يستطع جهاز التشغيل الوصول إلى السيرفر أو تسجيل الدخول، يطبع معرّف جهاز التشغيل ويطلب منك إلغاءه من لوحة التحكم. بعد ذلك يحذف الخدمة وملف الـ drop-in، و`/var/lib/dhow-runner` (الهوية، والـ credential، والـ journal)، و`/etc/dhow-runner`، والملف التنفيذي. يبقى المستخدم `dhow-runner` والـ sub-ids الخاصة به، إلا إذا أضفت `--remove-user`، وهو يحذف المستخدم فقط إذا كان `install` قد أنشأه (يذكر ذلك تعليق المستخدم). أما المستخدم الذي كان موجودًا من قبل فيبقى، ويخبرك `install` عندما يستخدم مستخدمًا موجودًا. وإذا شغّلته مرة أخرى لا يجد شيئًا يحذفه. الخيار `--yes` يتخطى السؤال.

## الإعداد اليدوي

الأمر `install` يفعل كل هذا بدلًا منك. وإذا أردت أن تفعله بنفسك:

```sh
sudo useradd --system --home-dir /var/lib/dhow-runner --shell /usr/sbin/nologin dhow-runner
sudo usermod --add-subuids 1048576-2097151 --add-subgids 1048576-2097151 dhow-runner
sudo install -d -m 0700 -o dhow-runner -g dhow-runner /var/lib/dhow-runner
sudo install -d -m 0750 -o root -g dhow-runner /etc/dhow-runner
sudo install -m 0640 -o root -g dhow-runner /dev/null /etc/dhow-runner/runner.env
```

- نطاق الـ sub-ids يعطي جهاز التشغيل 1048576 معرّفًا، وهذا يكفي 16 تشغيلًا في الوقت نفسه (65536 لكل تشغيل). اختر نطاقًا لا يتداخل معه أي سطر آخر في `/etc/subuid` و`/etc/subgid`.
- `/var/lib/dhow-runner` هو مجلد الحالة. فيه الهوية (المفاتيح الخاصة)، وسجل الحجوزات (journal)، ومجلد لكل محاولة تعمل. يرفض جهاز التشغيل البدء إذا استطاع مستخدمون آخرون الكتابة فيه، أو إذا كان ملف `identity` أو `journal` مقروءًا لأي أحد غير مستخدم جهاز التشغيل.
- `/etc/dhow-runner/runner.env` فيه [الإعدادات](configuration.md). يمكن أن يبقى فارغًا. ليس فيه أسرار، لكن اجعل الكتابة فيه لـ root فقط. ترفض الخدمة البدء بدونه.

ثم [سجّل جهاز التشغيل](register.md#2-التسجيل-على-جهاز-التشغيل) وشغّله بالأمر `sudo systemctl enable --now dhow-runner`. التسجيل اليدوي يحفظ الهوية في ملف عادي بالصلاحية `0600`، وتشغيل `sudo dhow-runner install` بعد ذلك يشفّرها.

## خدمة systemd

يأتي الإصدار بـ [`deploy/systemd/dhow-runner.service`](../../deploy/systemd/dhow-runner.service). تشغّل `dhow-runner run` بالمستخدم `dhow-runner`، وتقرأ `/etc/dhow-runner/runner.env`، وتعيد التشغيل عند الفشل، وتعطي جهاز التشغيل 60 ثانية ليتوقف. لا تعدّل الملف الذي يأتي مع الإصدار. غيّره بملف إضافي (drop-in) عبر `sudo systemctl edit dhow-runner`، مثلًا لتضيف `ReadWritePaths=` عندما لا يكون `DHOW_RUNNER_STATE_DIR` هو `/var/lib/dhow-runner`.

ما تضبطه الخدمة، ولماذا:

- `Delegate=yes` يسلّم شجرة cgroup الفرعية لجهاز التشغيل، حتى يضبط مشغّل Podman حدودًا لكل تشغيل. و`KillMode=control-group` يقتل كل عمليات التشغيلات مع جهاز التشغيل.
- `MemorySwapMax=0` يبعد ذاكرة التشغيلات، والـ tmpfs الذي يحفظ ملفات الأسرار، عن الـ swap. و`LimitCORE=0` يبعد المفاتيح عن core dumps.
- `StateDirectory=dhow-runner` مع `StateDirectoryMode=0700`: القيمة الافتراضية في systemd، `0755`، ستجعل جهاز التشغيل يرفض البدء.
- عند `systemctl stop` يتوقف جهاز التشغيل عن أخذ العمل، ويقتل البيئات المعزولة التي تعمل، ويبلغ عنها كمفقودة خلال 15 ثانية، حتى يعيد السيرفر محاولتها في مكان آخر.

### التحصين

يقيّم `systemd-analyze security dhow-runner.service` الخدمة بـ **5.2 MEDIUM** (systemd 255 و257 و259)، ويفشل CI إذا تجاوز التقييم 6.9. المستوى أقل من مستوى السيرفر لأن جهاز التشغيل يدير حاويات: Podman بلا root يحتاج إلى `newuidmap`، وهو أداة setuid (أو بـ file capabilities)، وإلى كل أنواع الـ namespaces. جرّبنا كل توجيه أدناه مع Podman بلا root يشغّل صورة تشغيل داخل الخدمة، ويشغّل CI مجموعتي اختبار الهروب والحدود داخل العزل نفسه.

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

الخطوة التالية: [افحص جهاز التشغيل](doctor.md)، ثم [شغّله](run.md).
