package output

var arMessages = map[Key]string{
	KeyUsage: `الاستخدام:
  dhow lint [--format text|json] [--lang en|ar] [path]
  dhow version
  dhow help

يفحص dhow lint ملف .dhow.yml بقواعد سيرفر داو نفسها، دون اتصال بالشبكة.
المسار الافتراضي هو .dhow.yml في المجلد الحالي. وإذا كان المسار مجلدًا، فيفحص dhow lint ملف .dhow.yml الذي فيه.

أكواد الخروج:
  0  الملف صحيح (وقد تكون فيه تحذيرات)
  1  في الملف أخطاء، والسيرفر يرفضه
  2  لا يمكن قراءة الملف، أو الأمر غير صحيح
`,
	KeyHint:           "شغّل dhow help لترى الأوامر.",
	KeyUnknownCommand: "dhow: %s ليس أمرًا.",
	KeyUnknownOption:  "dhow lint: %s ليس خيارًا.",
	KeyOptionValue:    "dhow lint: يحتاج %s إلى قيمة.",
	KeyFormatInvalid:  "dhow lint: قيمة --format هي text أو json.",
	KeyLangInvalid:    "dhow lint: قيمة --lang هي en أو ar.",
	KeyTooManyPaths:   "dhow lint: اكتب مسارًا واحدًا فقط.",
	KeyFileNotFound:   "dhow lint: %s: الملف غير موجود.",
	KeyFileNotRegular: "dhow lint: %s: هذا ليس ملفًا عاديًا.",
	KeyFileNoAccess:   "dhow lint: %s: ليست لديك صلاحية قراءة هذا الملف.",
	KeyFileUnreadable: "dhow lint: %s: لا يمكن قراءة الملف.",
	KeyValid:          "%s: خطوات النشر صحيحة.",
	KeyValidWarnings:  "%s: خطوات النشر صحيحة، وفيها تحذيرات عددها %d.",
	KeyInvalid:        "%s: الأخطاء %d، والتحذيرات %d. يرفض السيرفر خطوات النشر هذه.",
	KeySeverityError:  "خطأ",
	KeySeverityWarn:   "تحذير",
	KeyAt:             "في",
}
