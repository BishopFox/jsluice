fetch('https://httpbin.org/anything', {
  method: "POST",
  headers: {
    "Content-Type": "application/json"
  },
  body: JSON.stringify({id: 123})
}).then(r => r.json()).then(j => {
  window.output.innerText = JSON.stringify(j)
})  
