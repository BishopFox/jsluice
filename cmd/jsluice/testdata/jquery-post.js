$("button").click(function(){
  $.post("demo_test_post.asp",
  {
    name: "Donald Duck",
    city: "Duckburg"
  },
  function(data, status){
    alert("Data: " + data + "\nStatus: " + status);
	document.location = data.nextURL
  });
});

$("#logout").click(function(){
    $.get("/logout.php?rememberUser=true", {"redirect": "/logged-out.php", "rememberUser": true}, function(data, status){
        location.href = data.redirect;
        location.href = ["one", "two", data.redirect].join("/");
    })

});

$.post({url: "/api/v1/users", "redirect": "/logged-out.php", "data":{user: 123}}, function(data, status){
    location.href = data.redirect;
})

$.ajax({method: "PUT", url: "/api/v1/posts", "data":{post: 324}, headers: {"Content-Type": "application/json", "x-backend": "prod"}}, function(data, status){
    location.href = data.redirect;
})

