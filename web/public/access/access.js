(() => {
  const form = document.querySelector('#access-form')
  const input = document.querySelector('#invite-code')
  const feedback = document.querySelector('#access-feedback')
  const button = form?.querySelector('button')

  if (!form || !input || !feedback || !button) return

  const setFeedback = (message) => {
    feedback.textContent = message
  }

  form.addEventListener('submit', async (event) => {
    event.preventDefault()
    const inviteCode = input.value.trim()
    if (!inviteCode) {
      setFeedback('请输入邀请码。')
      input.focus()
      return
    }

    button.disabled = true
    setFeedback('正在验证…')
    try {
      const response = await fetch('/api/v1/access/verify', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', Accept: 'application/json' },
        body: JSON.stringify({ invite_code: inviteCode }),
      })
      if (response.status === 204) {
        window.location.assign('/')
        return
      }
      setFeedback(response.status === 429 ? '尝试次数过多，请稍后再试。' : '邀请码不正确，请重试。')
    } catch {
      setFeedback('暂时无法完成验证，请稍后再试。')
    } finally {
      button.disabled = false
    }
  })
})()
