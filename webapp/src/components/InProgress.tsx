import React from "react";
import CircularProgress from "@material-ui/core/CircularProgress";
import Backdrop from "@material-ui/core/Backdrop";
// import { styled } from "@material-ui/core/styles";
import styled from "@emotion/styled";
import { useTheme } from "@material-ui/core/styles";
export type InProgressProps = {
    open: boolean;
};

function InProgress(props: InProgressProps) {
    const defaultTheme = useTheme()
    const StyledBackdrop = styled(Backdrop)(() => {
        return {
            position: "absolute",
            zIndex: defaultTheme.zIndex.drawer + 1,
            color: "#fff",
        };
    });

    return (
        <StyledBackdrop open={props.open}>
            <CircularProgress />
        </StyledBackdrop>
    );
}

export default React.memo(InProgress);
