import React from "react"
import BookIcon from '@material-ui/icons/Book';
import styled from "@emotion/styled";

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

     const StyledIcon = styled(BookIcon)({
         width:"100%",
         height:"100%",
         color:"gainsboro",
       })     


     return (
        //next phase adding image
        // <img src={props.src} onError={handleError} />
       <StyledIcon/>
     )

  }

  export default React.memo(Image)
