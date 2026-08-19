(function () {
  var boot = document.getElementById('boot');
  var panel = document.getElementById('panel');
  var stepMode = document.getElementById('step-mode');
  var stepRemote = document.getElementById('step-remote');
  var stepLocal = document.getElementById('step-local');
  var errorEl = document.getElementById('error');
  var statusEl = document.getElementById('status');
  var dataDir = document.getElementById('data-dir');
  var remoteURL = document.getElementById('remote-url');
  var tray = document.getElementById('tray');

  function api() {
    var g = window.go;
    if (g && g.desktop && g.desktop.App) {
      return g.desktop.App;
    }
    throw new Error('Desktop API is not available.');
  }

  function showError(msg) {
    if (!msg) {
      errorEl.hidden = true;
      errorEl.textContent = '';
      return;
    }
    errorEl.hidden = false;
    errorEl.textContent = String(msg);
  }

  function showStatus(msg) {
    if (!msg) {
      statusEl.hidden = true;
      statusEl.textContent = '';
      return;
    }
    statusEl.hidden = false;
    statusEl.textContent = msg;
  }

  function showStep(step) {
    stepMode.hidden = step !== stepMode;
    stepRemote.hidden = step !== stepRemote;
    stepLocal.hidden = step !== stepLocal;
  }

  function selectedMode() {
    var on = document.querySelector('input[name="mode"]:checked');
    return on ? on.value : 'local';
  }

  function busy(on) {
    document.querySelectorAll('button').forEach(function (b) {
      b.disabled = on;
    });
  }

  function trayOn() {
    return tray.checked;
  }

  document.getElementById('next').addEventListener('click', function () {
    showError('');
    if (selectedMode() === 'remote') {
      showStep(stepRemote);
      remoteURL.focus();
      return;
    }
    showStep(stepLocal);
  });

  document.getElementById('back-remote').addEventListener('click', function () {
    showError('');
    showStep(stepMode);
  });
  document.getElementById('back-local').addEventListener('click', function () {
    showError('');
    showStep(stepMode);
  });

  document.getElementById('connect-remote').addEventListener('click', function () {
    var url = remoteURL.value.trim();
    if (!url) {
      showError('Enter the address of a Carrel instance.');
      remoteURL.focus();
      return;
    }
    busy(true);
    showStatus('Connecting…');
    showError('');
    api().ConnectRemote(url, trayOn()).catch(function (err) {
      busy(false);
      showStatus('');
      showError(err);
    });
  });

  document.getElementById('connect-local').addEventListener('click', function () {
    busy(true);
    showStatus('Starting the local server…');
    showError('');
    api().ConnectLocal(trayOn()).catch(function (err) {
      busy(false);
      showStatus('');
      showError(err);
    });
  });

  function init() {
    var host;
    try {
      host = api();
    } catch (err) {
      boot.textContent = String(err.message || err);
      return;
    }
    var href = location.href;
    var ready = host.RememberShell(href);
    var infoP = host.Info();
    Promise.all([ready, infoP]).then(function (parts) {
      var info = parts[1] || {};
      var dir = info.dataDir || info.DataDir || '';
      var needs = info.needsOnboarding;
      if (needs === undefined) {
        needs = info.NeedsOnboarding;
      }
      var hostErr = info.error || info.Error || '';
      if (dir) {
        dataDir.value = dir;
      }
      if (hostErr) {
        showError(hostErr);
      }
      if (!needs) {
        boot.textContent = 'Loading…';
        return;
      }
      boot.hidden = true;
      panel.hidden = false;
    }).catch(function (err) {
      boot.textContent = String(err);
    });
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }
})();
