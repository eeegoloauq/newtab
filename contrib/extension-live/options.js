var field = document.getElementById('url');
var state = document.getElementById('state');
var tryIt = document.getElementById('try');
var colour = document.getElementById('colour');
var colourState = document.getElementById('colourstate');

// Local answers immediately and is what a new tab reads; sync is what
// carries the address to your other machines.
chrome.storage.local.get({ url: '', colour: '' }, function (local) {
  colour.value = local.colour;
  if (local.url) {
    show(local.url);
    return;
  }
  chrome.storage.sync.get({ url: '', colour: '' }, function (synced) {
    colour.value = synced.colour;
    show(synced.url);
  });
});

// Asking the page what colour it is, instead of asking the reader to
// type it: the address is one permission prompt away, and the page
// publishes the colour in its manifest and in a meta tag. Refused
// permission is not an error — the field below still works by hand.
function learnColour(url) {
  var origin;
  try {
    origin = new URL(url).origin + '/*';
  } catch (e) {
    return;
  }
  chrome.permissions.request({ origins: [origin] }, function (granted) {
    if (!granted) {
      colourState.textContent = 'not allowed to ask the page — set it below if you like';
      return;
    }
    fetch(new URL('/manifest.webmanifest', url).href)
      .then(function (r) { return r.json(); })
      .then(function (doc) {
        var found = doc.background_color || doc.theme_color || '';
        if (!/^#[0-9a-f]{6}$/i.test(found)) { throw new Error('no colour'); }
        colour.value = found;
        chrome.storage.local.set({ colour: found });
        chrome.storage.sync.set({ colour: found });
        colourState.textContent = 'taken from the page: ' + found;
        if (doc.background_image) {
          keepPicture(new URL(doc.background_image, url).href);
        } else {
          // No picture any more: forget the one we were holding.
          chrome.storage.local.remove('picture');
        }
      })
      .catch(function () {
        colourState.textContent = 'the page did not say — set it below if you like';
      });
  });
}

// The picture itself, kept here rather than fetched on every new tab.
// The address carries the file's digest, so a changed wallpaper is a
// changed address and this replaces itself; an unchanged one is never
// fetched twice. Only in local storage: sync has room for settings, not
// for photographs.
function keepPicture(href) {
  chrome.storage.local.get({ picture: null }, function (held) {
    if (held.picture && held.picture.href === href) {
      return;
    }
    fetch(href)
      .then(function (r) { return r.blob(); })
      .then(function (blob) {
        if (blob.size > 4 * 1024 * 1024) { throw new Error('too big to hold'); }
        return new Promise(function (ok, no) {
          var reader = new FileReader();
          reader.onload = function () { ok(reader.result); };
          reader.onerror = no;
          reader.readAsDataURL(blob);
        });
      })
      .then(function (data) {
        chrome.storage.local.set({ picture: { href: href, data: data } });
      })
      .catch(function () { chrome.storage.local.remove('picture'); });
  });
}

colour.addEventListener('input', function () {
  var value = colour.value.trim();
  if (value !== '' && !/^#[0-9a-f]{6}$/i.test(value)) {
    colourState.textContent = 'six hex digits, like #141312';
    return;
  }
  chrome.storage.local.set({ colour: value });
  chrome.storage.sync.set({ colour: value }, function () {
    colourState.textContent = value === '' ? 'browser default' : 'saved';
  });
});

function show(url) {
  field.value = url;
  tryIt.disabled = url === '';
  state.classList.remove('bad');
  state.textContent = url === '' ? 'not set yet' : 'saved — new tabs open this';
}

field.addEventListener('input', function () {
  var url = field.value.trim();
  if (url !== '' && !/^https?:\/\//i.test(url)) {
    // A new tab that redirects to whatever fits in this box is not
    // something to allow quietly.
    state.classList.add('bad');
    state.textContent = 'needs to start with http:// or https://';
    tryIt.disabled = true;
    return;
  }
  chrome.storage.local.set({ url: url });
  chrome.storage.sync.set({ url: url }, function () {
    show(url);
    if (url !== '') { learnColour(url); }
  });
});

tryIt.addEventListener('click', function () {
  // Opening a tab is how you find out whether the address was right, and
  // it is the next thing anyone does anyway.
  chrome.tabs.create({});
  window.close();
});
