import CircularProgress from "@material-ui/core/CircularProgress";
import Backdrop from "@material-ui/core/Backdrop";
import { styled } from "@material-ui/core/styles";

export type InProgressProps = {
    open: boolean;
};

export default function InProgress(props: InProgressProps) {
    const StyledBackdrop = styled(Backdrop)(({ theme }) => ({
        position: "absolute",
        zIndex:theme.zIndex.drawer + 1,
        color: '#fff',
    }));

    return (
        <StyledBackdrop open={props.open}>
            <CircularProgress />
        </StyledBackdrop>
    );
}
