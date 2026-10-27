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

  var SYSTEM = ['sea', 'teak', 'dusk', 'indigo'];
  var HEX = /^#[0-9a-f]{6}$/i;
  var ID = /^[a-z0-9-]{1,40}$/;
  var VARIABLES = {
    canvas: '--color-canvas',
    rail: '--color-rail',
    input: '--color-input',
    inset: '--color-surface-inset',
    surface: '--color-surface',
    raised: '--color-surface-raised',
    top: '--color-surface-top',
    track: '--color-track',
    t1: '--color-primary',
    t2: '--color-secondary',
    t3: '--color-tertiary',
    t4: '--color-muted',
    a1: '--color-aurora-1',
    a2: '--color-aurora-2',
    a3: '--color-aurora-3',
    a4: '--color-aurora-4',
    a5: '--color-aurora-5',
    a6: '--color-aurora-6',
    'edge-warm': '--color-edge-warm',
    'edge-cool': '--color-edge-cool',
    green: '--color-accent-green',
    blue: '--color-accent-blue',
    amber: '--color-accent-amber',
    coral: '--color-accent-coral',
    'violet-soft': '--color-accent-violet-soft',
  };

  function findCustom(id) {
    var list = JSON.parse(window.localStorage.getItem('dhow.themes') || '[]');
    if (!Array.isArray(list)) return null;
    for (var index = 0; index < list.length; index += 1) {
      var entry = list[index];
      if (entry && typeof entry === 'object' && entry.Id === id && entry.Colors && typeof entry.Colors === 'object') {
        return entry.Colors;
      }
    }
    return null;
  }

  try {
    var active = window.localStorage.getItem('dhow.theme');
    if (SYSTEM.indexOf(active) >= 0) {
      root.setAttribute('data-theme', active);
    } else if (typeof active === 'string' && ID.test(active)) {
      var colors = findCustom(active);
      if (colors) {
        var applied = 0;
        Object.keys(VARIABLES).forEach(function (key) {
          var value = Object.prototype.hasOwnProperty.call(colors, key) ? colors[key] : null;
          if (typeof value === 'string' && HEX.test(value)) {
            root.style.setProperty(VARIABLES[key], value.toLowerCase());
            applied += 1;
          }
        });
        if (applied === 0) root.setAttribute('data-theme', 'sea');
      }
    }
  } catch (error) {
    root.setAttribute('data-theme', 'sea');
  }
})();
