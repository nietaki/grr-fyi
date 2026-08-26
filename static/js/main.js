document.addEventListener('alpine:init', () => {
  console.log('Alpine is initializing...');

  Alpine.data('combined', () => ({}));

  Alpine.data('createLink', () => ({
    targetUrl: '',
    customSlug: '',
    loading: false,
    error: null,

    async submit() {
      this.loading = true;
      this.error = null;

      try {
        const res = await fetch('/_/api/create_link', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            target_url: this.targetUrl,
            custom_slug: this.customSlug || undefined,
          }),
        });
        const data = await res.json();
        if (res.ok) {
          window.location.href = '/_/edit_link/' + data.slug + '?claim_key=' + data.claim_key;
        } else {
          this.error = data.error;
        }
      } catch {
        this.error = 'Something went wrong';
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
