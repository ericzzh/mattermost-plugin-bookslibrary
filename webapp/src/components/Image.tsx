import React from "react"

type ImageProps = {
    src:string,
    handleError?: ()=>void,
  }

function Image(props:ImageProps){

     const [state, setState] = React.useState({
          errored: false,
       })

     const handleError = ()=>{
         if(!props.handleError || state.errored) return
         setState({
             errored:true
           })
         props.handleError()
       }


     return (
       <img src={props.src} onError={handleError} />
     )

  }

  export default React.memo(Image)
