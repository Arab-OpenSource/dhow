(function () {
  var root = document.documentElement;
  var lang = 'ar';
  try {
    if (window.localStorage.getItem('dhow.lang') === 'en') lang = 'en';
  } catch (error) {
    lang = 'ar';
  }
  root.lang = lang;
  root.dir = lang === 'en' ? 'ltr' : 'rtl';
})();
