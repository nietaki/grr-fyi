document.addEventListener('alpine:init', () => {
  console.log('Alpine is initializing...');

  Alpine.data('combined', () => ({}));

  Alpine.data('createLink', () => ({
    targetUrl: '',
    customSlug: '',
    loading: false,
    error: null,
    // Name of the form field the last error refers to ("target_url",
    // "custom_slug" or null); mirrors ErrorResponse.field from the API.
    errorField: null,
    captchaPayload: null,
    captchaRequired: false,

    init() {
      this.captchaRequired = !!document.querySelector('altcha-widget');
    },

    async submit() {
      if (this.captchaRequired && !this.captchaPayload) {
        this.error = 'Please complete the captcha';
        this.errorField = null;
        return;
      }

      this.loading = true;
      this.error = null;
      this.errorField = null;

      try {
        const body = {
          target_url: this.targetUrl,
          custom_slug: this.customSlug || undefined,
        };
        if (this.captchaPayload) {
          body.altcha = this.captchaPayload;
        }

        const res = await fetch('/_/api/create_link', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(body),
        });
        const data = await res.json();
        if (res.ok) {
          window.location.href = '/_/edit_link/' + data.slug + '?claim_key=' + data.claim_key;
        } else {
          this.error = data.error;
          this.errorField = data.field || null;
        }
      } catch {
        this.error = 'Something went wrong';
        this.errorField = null;
      } finally {
        this.loading = false;
      }
    },
  }));

  Alpine.data('editLink', () => ({
    copied: false,

    copyShortUrl() {
      const input = this.$refs.shortUrl;
      navigator.clipboard.writeText(input.value).then(() => {
        this.copied = true;
        setTimeout(() => { this.copied = false; }, 2000);
      });
    },
  }));
});
