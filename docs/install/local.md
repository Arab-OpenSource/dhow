[English](../en/install/local.md)

# شغّل داو على جهازك

هذه الصفحة لتجربة داو (Dhow) أو العمل عليه على حاسوبك. لتثبيته لأشخاص آخرين، استخدم [Docker Compose](compose.md) خلف [وكيل عكسي](reverse-proxy.md).

> [!WARNING]
> **`DHOW_DEV=true` لجهازك أنت فقط.** يسمح لداو بالبدء بعنوان عام `http://` واتصال بقاعدة البيانات بدون TLS. لذلك تعبر كلمات السر وكوكيز الجلسة وكل الأسرار الشبكة بنص عادي. في هذا الوضع يكتب داو في كل تشغيل: `DHOW_DEV=true: plain HTTP and unencrypted database connections are allowed. Never use this in production.` لا تضبطه أبدًا على سيرفر، أو جهاز مشترك، أو أي شيء يصل إليه غيرك. ولا تنسخ أبدًا ملف `.env` الخاص بالتطوير إلى الإنتاج.

هناك طريقتان لتشغيل داو على جهازك:

| | [من الكود المصدري](#من-الكود-المصدري) | [حزمة Compose](#حزمة-compose) |
|---|---|---|
| لمن | للعمل على داو | لتجربة الصورة المنشورة |
| تحتاج إلى | Go، وNode.js LTS مع pnpm، وPostgreSQL 18 (Docker أو Podman أو تثبيت مباشر) | Docker مع Compose |
| العنوان | `http://localhost:8080` | `http://127.0.0.1:8080`، وHTTPS لتسجيل الدخول |
| `DHOW_DEV` | `true` | غير مضبوط |

## المتصفحات و`http://localhost`

كوكي الجلسة `__Host-dhow_session` دائمًا `Secure`. المتصفحات تحفظ الكوكي `Secure` فقط من سياق آمن (secure context)، وChrome وFirefox يعتبران `http://localhost` و`http://127.0.0.1` سياقًا آمنًا. لذلك يعمل تسجيل الدخول عبر HTTP العادي على الجهاز نفسه، في هذين المتصفحين. ولا يعمل من جهاز آخر على شبكتك، مثل `http://192.168.1.20:8080`. هذا العنوان ليس سياقًا آمنًا، فيتجاهل المتصفح الكوكي، ويبدو أن تسجيل الدخول لا يفعل شيئًا. استخدم HTTPS في هذه الحالة.

يرسل المتصفح أيضًا العنوان الذي في الشريط كـ `Origin`، ويقارنه داو بـ `DHOW_PUBLIC_URL` حرفًا بحرف. `http://localhost:8080` و`http://127.0.0.1:8080` مصدران مختلفان. افتح العنوان الذي ضبطته بالضبط، وإلا يفشل الإعداد وتسجيل الدخول بـ `CSRF_FAILED`.

## من الكود المصدري

الدليل الكامل للعمل على داو هو [طوّر داو على جهازك](../development.md): حزمة Docker فقط، والأدوات المباشرة على Linux وmacOS وWindows (WSL2)، والواجهة مع إعادة التحميل المباشر، والاختبارات، وجهاز التشغيل. إذا كان لديك Git وDocker فقط:

```sh
docker compose -f deploy/dev/compose.yaml up
```

ثم افتح الرابط الذي يطبعه السجل في الإطار، على `http://localhost:5173`. ومع الأدوات المباشرة:

```sh
make dev-env
make dev-db
make web
make dev
```

ثم افتح رابط `setup_url` الذي يكتبه السيرفر في السجل، على `http://localhost:8080`.

## حزمة Compose

هذه الطريقة تشغّل الصورة المنشورة كما في التثبيت الحقيقي. لذلك `DHOW_DEV` غير مضبوط، والاتصال بقاعدة البيانات يستخدم TLS.

1. اتبع [البدء السريع مع Compose](compose.md#البدء-السريع)، واترك العنوان العام الافتراضي:

   ```sh
   ./init.sh
   docker compose up -d
   ```

   يضبط `init.sh` القيمة `DHOW_PUBLIC_URL=https://dhow.localhost`. الأسماء تحت `.localhost` تشير إلى جهازك.

2. تحقق منه بـ `curl -s http://127.0.0.1:8080/api/v1/health/ready`، وافتح `http://127.0.0.1:8080` لترى الواجهة.

3. الإعداد وتسجيل الدخول يحتاجان إلى المتصفح على `https://dhow.localhost`، لأن هذا هو `DHOW_PUBLIC_URL` الذي يجب أن يطابقه `Origin`. ضع [Caddy](https://caddyserver.com/docs/install) أمامه. مع اسم تحت `.localhost`، ينشئ Caddy سلطة شهادات محلية، ويطلب كلمة سرك مرة واحدة ليضيفها إلى قائمة الثقة في النظام:

   ```sh
   sudo caddy reverse-proxy --from dhow.localhost --to 127.0.0.1:8080
   ```

   ثم افتح رابط الإعداد من `docker compose logs dhow | grep setup_url`.

`caddy reverse-proxy` أمر سريع لجهازك فقط. للسيرفر الحقيقي، استخدم إعدادًا من [ضع داو خلف وكيل عكسي](reverse-proxy.md).
