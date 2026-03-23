export default function Downloads() {
  return (
    <div class="downloads-page">
      <h2>Get SelfShare Apps</h2>
      <p class="downloads-subtitle">Access your files from any device</p>

      <div class="download-cards">
        <div class="download-card">
          <div class="download-icon">{'\u{1F4F1}'}</div>
          <h3>Android</h3>
          <p>File browser, photo gallery, video player, and camera upload.</p>
          <a href="/app/download" class="btn-primary">Download APK</a>
          <div class="download-note">
            Open this page on your Android phone to download directly,
            or scan the QR code from your phone's browser.
          </div>
        </div>

        <div class="download-card">
          <div class="download-icon">{'\u{1F34E}'}</div>
          <h3>iOS</h3>
          <p>Coming soon. Use the web app in Safari for now.</p>
          <button class="btn-secondary" disabled>Coming Soon</button>
          <div class="download-note">
            Tip: In Safari, tap Share then "Add to Home Screen" for an app-like experience.
          </div>
        </div>

        <div class="download-card">
          <div class="download-icon">{'\u{1F310}'}</div>
          <h3>Web App</h3>
          <p>You're using it right now! Works on any device with a browser.</p>
          <div class="download-note">
            Bookmark this page or add it to your home screen.
          </div>
        </div>
      </div>
    </div>
  );
}
