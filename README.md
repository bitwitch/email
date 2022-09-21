# Send email via Google's SMTP server using OAuth2

### Why this package exists
My use case is I want to send email programatically. Many of my programs that  
run daily have code paths that send email to certain members of my team when  
errors occur or to notify them of certain events.  

In the past you could just use a gmail username and password and very easily  
send email via google's SMTP server. However, Google recently deprecated simple  
username and password authentication, so now the only real option for  
programatically sending email from a gmail account is oauth2. Go has an SMTP  
package, but it does not support authenticating with the SMTP server via OAUTH.  
So I wrote this little package to allow my programs to send email from a gmail  
account that I control.  

### Setup
You must generate a credentials file in order to be able to authenticate with  
the SMTP server. This involves setting up an "app" on google, setting the  
appropriate permissions scope, and then granting the app permision to send  
email via a particular gmail account.  

1. In [google cloud console](https://console.cloud.google.com) create a new App  

2. Create a new OAuth2 Client ID  
    - APIs & Services -> Credentials -> Create Credentials -> OAuth2 Client ID  

3. Set up the OAuth consent screen
    - Here you can setup the redirect url where you want google to send users  
      after they grant permission to your app (note that in my case, there  
      arent really external "users", "users" is just me using the one gmail  
      account I want to send email from).   
    - You probably want to setup a small site that parses the query parameters  
      and constructs a credentials file out of it. I made a [page](https://otto.pixel.nyc/home/google-grant) on the Otto  
      Site that does this.  
    - You must also set the scopes that your app needs access to here. Sending  
      email via SMTP requires the restricted scope `https://mail.google.com/`  

4. Make sure your app has access to restricted scopes
    - In order to use restricted scopes, the administrator of the google  
      workspace must [grant internal apps permission to use restricted google  
      workspace apis](https://support.google.com/a/answer/7281227?authuser=1#homegrown&zippy=%2Cstep-manage-third-party-app-access-to-google-services-add-apps%2Cstep-control-api-access). (This implies that you want to configure the app to be an  
      internal app that is only used by an internal team and not meant for  
      public users).  

5. Grant the app permission to access a gmail account
    - Go through the OAuth2 flow and obtain the required credentials: ClientId,  
      ClientSecret, and RefreshToken.  

### Usage
Remember that you must first make a single call to Init and pass in the from  
address and a path to the credentials file. This initializes the email client.  
Then you can simply call Send to send emails. You can send attachments as well  
by passing in an array of filepaths as the 'attachments' argument to Send.  

```go
import "github.com/bitwitch/email"

func main() {
    email.Init("the_from_address@email.com", "path_to_credentials_file.json")

    subject := "Subject"
    messages := []string{"message number one", "message number two"}
    attachments := []string{}
    recipients := []string{"first_recipient@email.com"}

    err := email.Send(subject, messages, attachments, recipients)
    if err != nil {
        panic(err)
    }
}
```
