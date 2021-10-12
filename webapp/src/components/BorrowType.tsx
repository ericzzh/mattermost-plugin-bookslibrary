import React from "react";
import PropTypes from "prop-types";
import Grid from "@material-ui/core/Grid";
import Paper from "@material-ui/core/Paper";
import TextField from "@material-ui/core/TextField";
import Chip from "@material-ui/core/Chip";
import AlarmOnIcon from "@material-ui/icons/AlarmOn";
import Button from "@material-ui/core/Button";
import IconButton from "@material-ui/core/IconButton";
import DeleteIcon from "@material-ui/icons/Delete";
import { makeStyles, withStyles, styled } from "@material-ui/core/styles";
// import { getCurrentTeam } from "mattermost-redux/selectors/entities/teams";
import { useSelector } from "react-redux";
import WorkerIcon from "@material-ui/icons/PermContactCalendar";
import BorrowIcon from "@material-ui/icons/MenuBook";
import HouseIcon from "@material-ui/icons/House";
import { useDispatch } from "react-redux";
import { GlobalState } from "mattermost-redux/types/store";
import moment from "moment";
import manifest from "../manifest";
import { getConfig } from "mattermost-redux/selectors/entities/general";
import Avatar from "@material-ui/core/Avatar";
// import Avatar from '@mui/material/Avatar';
import Stepper from "@material-ui/core/Stepper";
import Step from "@material-ui/core/Step";
import StepLabel from "@material-ui/core/StepLabel";
import StepConnector from "@material-ui/core/StepConnector";
import { StepIconProps } from "@material-ui/core/StepIcon";
import clsx from "clsx";
import useMediaQuery from "@material-ui/core/useMediaQuery";
import { useTheme } from "@material-ui/core/styles";
import Accordion from "@material-ui/core/Accordion";
import AccordionSummary from "@material-ui/core/AccordionSummary";
import AccordionDetails from "@material-ui/core/AccordionDetails";
import ExpandMoreIcon from "@material-ui/icons/ExpandMore";
import {
    fetchConfig,
    getExpireDays,
    getMaxRenewTimes,
    getStatus,
    getError,
} from "../ConfigSlice";
import { getCurrentUser } from "mattermost-redux/selectors/entities/common";
import { Client4 } from "mattermost-redux/client";
import InProgress from "./InProgress";
import MsgBox, { MsgBoxProps } from "./MsgBox";

const STATUS_REQUESTED = "R";
const STATUS_CONFIRMED = "C";
const STATUS_DELIVIED = "D";
const STATUS_RENEW_REQUESTED = "RR";
const STATUS_RENEW_CONFIRMED = "RC";
const STATUS_RETURN_REQUESTED = "RTR";
const STATUS_RETURN_CONFIRMED = "RTC";
const STATUS_RETURNED = "RT";

const WORKFLOW_BORROW = "BORROW";
const WORKFLOW_RENEW = "RENEW";
const WORKFLOW_RETURN = "RETURN";

const { formatText, messageHtmlToComponent } = window.PostUtils;

const TEXT: Record<string, string> = {
    ["WF_" + WORKFLOW_BORROW]: "借书流程",
    ["WF_" + WORKFLOW_RENEW]: "续借流程",
    ["WF_" + WORKFLOW_RETURN]: "还书流程",
    ["ST_" + STATUS_REQUESTED]: "借书请求",
    ["ST_" + STATUS_CONFIRMED]: "借书确认",
    ["ST_" + STATUS_DELIVIED]: "已送出",
    ["ST_" + STATUS_RENEW_REQUESTED]: "续借请求",
    ["ST_" + STATUS_RENEW_CONFIRMED]: "续借确认",
    ["ST_" + STATUS_RETURN_REQUESTED]: "还书请求",
    ["ST_" + STATUS_RETURN_CONFIRMED]: "还书确认",
    ["ST_" + STATUS_RETURNED]: "已还书",
    ["DATE_" + STATUS_REQUESTED]: "借阅请求日",
    ["DATE_" + STATUS_CONFIRMED]: "借阅确认日",
    ["DATE_" + STATUS_DELIVIED]: "借阅送达日",
    ["DATE_" + STATUS_RENEW_REQUESTED]: "续借请求日",
    ["DATE_" + STATUS_RENEW_CONFIRMED]: "续借确认日",
    ["DATE_" + STATUS_RETURN_REQUESTED]: "还书请求日",
    ["DATE_" + STATUS_RETURN_CONFIRMED]: "还书确认日",
    ["DATE_" + STATUS_RETURNED]: "还书送达日",
    next_step: "下一步",
    date_accordion: "日期",
    CONFIRM_DELETE: "确定删除此借书请求吗？",
    ALERT_DELETE_ERROR: "删除失败。错误：",
    ALERT_DELETE_SUCC: "删除成功。",
    CONFIRM_STEP: `确定进入到状态：`,
    WORKFLOW_ERROR: "工作流请求失败。错误：",
    WORKFLOW_SUCC: "工作流请求成功。",
    LOAD_CONFIG_ERROR: "加载配置数据失败。错误：",
    REJECT: "拒绝",
};

function FindStatusInWorkflow(status: string, workflow: Step[]) {
    return workflow.find((step) => step.status === status);
}
function BorrowType(props: any) {
    const post = { ...props.post };
    const message = post.message || "";
    const config = useSelector(getConfig);
    // const currentTeam = useSelector(getCurrentTeam);
    const currentUser = useSelector(getCurrentUser);
    const theme = useTheme();
    const matchesSm = useMediaQuery(theme.breakpoints.up("sm"));
    // const matchesXs = useMediaQuery(theme.breakpoints.up("xs"));

    const expireDays = useSelector(getExpireDays);
    const maxRenewTimes = useSelector(getMaxRenewTimes);
    const loadStatus = useSelector(getStatus);
    const loadConfigError = useSelector(getError);
    const [loading, setLoading] = React.useState(false);
    const [msgBox, setMsgBox] = React.useState<MsgBoxProps>({
        open: false,
        text: "",
        serverity: "success",
    });
    const onCloseMsg = () => {
        setMsgBox({
            open: false,
            text: "",
            serverity: "success",
        });
    };

    if (loadConfigError) {
        setMsgBox({
            open: true,
            text: TEXT["LOAD_CONFIG_ERROR"] + loadConfigError,
            serverity: "error",
        });
    }

    const dispatch = useDispatch();

    React.useEffect(() => {
        if (expireDays === -1 || maxRenewTimes === -1) {
            !loading && setLoading(true);
            if (loadStatus === "loading") {
                return;
            }
            dispatch(fetchConfig);
        } else {
            loading && setLoading(false);
        }
    }, [expireDays, maxRenewTimes]);

    // const currentChannel = useSelector(getCurrentChannel);
    // const formattedText = messageHtmlToComponent(formatText(message));
    //
    // const dispatch = useDispatch();
    //

    let borrow: Borrow;
    let borrowReq: BorrowRequest;
    let currentStep: Step;
    let workflow: Step[];

    try {
        borrow = JSON.parse(message);
        borrowReq = borrow.dataOrImage;
        workflow = borrow.dataOrImage.workflow;
        currentStep = borrowReq.workflow[borrowReq.step_index];
    } catch (error) {
        const formattedText = messageHtmlToComponent(formatText(message));
        return <div> {formattedText} </div>;
    }

    const StyledImgWrapper = styled(Grid)(({ theme }) => ({
        [theme.breakpoints.up("xs")]: {
            width: 125 * 0.8,
            height: 160 * 0.8,
        },
        [theme.breakpoints.up("sm")]: {
            width: 125 * 1.5,
            height: 160 * 1.5,
        },
        "& img": {
            maxWidth: "100%",
            maxHeight: "100%",
            float: "left",
        },
    }));

    const StyledHeadStatus = styled(Avatar)(({ theme }) => ({
        headStatus: {
            backgroundColor: "red",
            [theme.breakpoints.up("xs")]: {
                width: "2rem",
                height: "2rem",
            },
            [theme.breakpoints.up("sm")]: {
                width: "2rem",
                height: "2rem",
            },
        },
    }));

    const checkExpire = () => {
        if (expireDays === -1) {
            return false;
        }

        function findStatusAndCheck(status: string) {
            const step = FindStatusInWorkflow(status, workflow);
            if (!step?.action_date) {
                return null;
            }
            const action_date = step.action_date;
            const expiredDate = moment(action_date).add(expireDays, "days");
            if (expiredDate > moment(Date.now())) {
                return true;
            }
            return false;
        }

        const rc = findStatusAndCheck(STATUS_RENEW_CONFIRMED)

        if(rc !== null) return rc

        const dlv = findStatusAndCheck(STATUS_DELIVIED)

        if(dlv !== null) return dlv

        return false
    };

    const headStatus = (
        <Grid container>
            <Grid item>
                {checkExpire() && (
                    <StyledHeadStatus>
                        <AlarmOnIcon />
                    </StyledHeadStatus>
                )}
            </Grid>
        </Grid>
    );

    const canDelete = () => {
        if (
            currentStep.status === STATUS_REQUESTED ||
            currentStep.status === STATUS_CONFIRMED ||
            currentStep.status === STATUS_RETURNED
        ) {
            return true;
        }

        return false;
    };

    const handleDelete = async () => {
        if (!confirm(TEXT["CONFIRM_DELETE"])) {
            return;
        }

        const request: WorkflowRequest = {
            master_key: borrow.relations_keys.master,
            act_user: currentUser.username,

            delete: true,
        };

        setLoading(true);
        const data = await Client4.doFetch<Result>(
            `/plugins/${manifest.id}/workflow`,
            {
                method: "POST",
                body: JSON.stringify(request),
            }
        );
        setLoading(false);

        if (data.error) {
            alert(TEXT["ALERT_DELETE_ERROR"] + data.error);
            console.error(data);
            return;
        }

        alert(TEXT["ALERT_DELETE_SUCC"]);
    };

    const buttonbar = (
        <Grid container justifyContent={"flex-end"}>
            {canDelete() && (
                <IconButton onClick={handleDelete}>
                    <DeleteIcon />
                </IconButton>
            )}
        </Grid>
    );

    const titleBar = (
        <Grid item container alignItems={"center"}>
            <Grid item xs={8}>
                {headStatus}
            </Grid>
            <Grid item xs={4}>
                {buttonbar}
            </Grid>
        </Grid>
    );

    // <Link href={`${config.SiteURL}/${currentTeam.id}/pl/${borrow.dataOrImage.book_post_id}`}>
    //     {"转到图书"}
    // </Link>

    const BookInfo = styled(Grid)(({ theme }) => ({
        "& .BookInfo": {
            [theme.breakpoints.up("xs")]: {
                marginBottom: "1rem",
            },
            [theme.breakpoints.up("sm")]: {
                marginBottom: "1.5rem",
            },
        },
        "& .BookName": {
            [theme.breakpoints.up("xs")]: {
                fontSize: "1.5rem",
                fontWeight: "bold",
                marginBottom: "1rem",
            },
            [theme.breakpoints.up("sm")]: {
                fontSize: "3rem",
                fontWeight: "bold",
                marginBottom: "2rem",
            },
        },
        "& .AuthorName": {
            [theme.breakpoints.up("xs")]: {
                fontSize: "1rem",
                fontWeight: "bold",
            },
            [theme.breakpoints.up("sm")]: {
                fontSize: "1.5rem",
                fontWeight: "bold",
            },
        },
        "& .Paticipant": {
            [theme.breakpoints.up("xs")]: {
                marginBottom: "1rem",
            },
            [theme.breakpoints.up("sm")]: {
                marginBottom: "4rem",
            },

            "& .PaticipantCommon": {
                "& .MuiChip-label": {
                    [theme.breakpoints.up("xs")]: {
                        fontSize: "0.8rem",
                    },
                    [theme.breakpoints.up("sm")]: {
                        fontSize: "0.8rem",
                    },
                },
                "& .MuiChip-root": {
                    [theme.breakpoints.up("xs")]: {
                        height: "2rem",
                    },
                    [theme.breakpoints.up("sm")]: {
                        height: "2rem",
                    },
                },
            },

            "& .PaticipantBorrower": {
                backgroundColor: "purple",
            },
            "& .PaticipantLibworker": {
                backgroundColor: "green",
            },
            "& .PaticipantKeeper": {
                backgroundColor: "teal",
            },
        },
    }));

    const participants = (
        <Grid container spacing={1}>
            <Grid item>
                <Chip
                    size={"medium"}
                    // variant={"outlined"}
                    icon={<BorrowIcon />}
                    color="primary"
                    label={borrow.dataOrImage.borrower_name}
                    className={"PaticipantCommon PaticipantBorrower"}
                />
            </Grid>
            <Grid item>
                <Chip
                    size={"medium"}
                    // variant={"outlined"}
                    icon={<WorkerIcon />}
                    color="primary"
                    label={borrow.dataOrImage.libworker_name}
                    className={"PaticipantCommon PaticipantLibworker"}
                />
            </Grid>
            <Grid item>
                {borrow.dataOrImage.keeper_names.map((keeper_name) => (
                    <Chip
                        size={"medium"}
                        // variant={"outlined"}
                        icon={<HouseIcon />}
                        color="primary"
                        label={keeper_name}
                        className={"PaticipantCommon PaticipantKeeper"}
                    />
                ))}
            </Grid>
        </Grid>
    );

    const requestInfo = (
        <BookInfo container direction="column">
            <Grid item className={"BookInfo"}>
                <div className={"BookName"}>{borrow.dataOrImage.book_name}</div>
                <div className={"AuthorName"}>{borrow.dataOrImage.author}</div>
            </Grid>
            <Grid item className={"Paticipant"}>
                {participants}
            </Grid>
        </BookInfo>
    );

    const StyledDateAccordion = styled(Accordion)(({ theme }) => ({
        width: "100%",
        "& .MuiAccordionSummary-root": {
            fontSize: "1.2rem",
        },
    }));

    const exploreWF = (cb: (s: Step) => boolean) => {
        // return bool value of cb
        const next = (step: Step, cb: (s: Step) => boolean) => {
            if (cb(step)) {
                return true;
            }

            const nextStepIndex = step.next_step_index;

            if (nextStepIndex === null) {
                return false;
            }

            for (let i of nextStepIndex) {
                const nextStep = workflow[i];
                if (
                    step.status === STATUS_RENEW_CONFIRMED &&
                    nextStep.status === STATUS_RENEW_REQUESTED
                ) {
                    return false;
                }

                if (next(nextStep, cb)) {
                    return true;
                }
            }

            return false;
        };

        next(workflow[0], cb);
    };

    const addDates = () => {
        const DateField = styled(TextField)(({ theme }) => ({
            "& label,input": {
                [theme.breakpoints.up("xs")]: {
                    fontSize: "1rem",
                },
                [theme.breakpoints.up("sm")]: {
                    fontSize: "1.5rem",
                },
            },
        }));

        let actionDates: React.ReactElement[] = [];

        exploreWF((step) => {
            if (!step.completed) {
                return true;
            }

            actionDates.push(
                <Grid item>
                    <DateField
                        label={TEXT["DATE_" + step.status]}
                        value={moment(step.action_date).format("YYYY-MM-DD")}
                        InputProps={{
                            readOnly: true,
                        }}
                    />
                </Grid>
            );

            return false;
        });

        return actionDates;
    };

    const dateOfRequest = (
        <Grid container>
            <StyledDateAccordion>
                <AccordionSummary
                    expandIcon={<ExpandMoreIcon />}
                    id="request_date"
                >
                    <div>{TEXT["date_accordion"]}</div>
                </AccordionSummary>
                <AccordionDetails>
                    <Grid container spacing={1}>
                        {addDates()}
                    </Grid>
                </AccordionDetails>
            </StyledDateAccordion>
        </Grid>
    );

    const StyledWFStepper = styled(Stepper)(({ theme }) => ({
        "& svg": {
            fontSize: "2rem",
        },
        "& .MuiStepLabel-label": {
            fontSize: "1.2rem",
        },
    }));

    const StyledWFButton = styled(Button)(({ theme }) => ({
        marginLeft: 10,
        marginTop: 10,
        width: "10rem",
        alignSelf: "flex-end",
    }));

    const StyledWFTypeChip = styled(Chip)(({ theme }) => ({
        width: "30%",
        height: "2rem",
        marginTop: "1.5rem",
        fontSize: "1.2rem",
    }));

    let localWorkflow: Step[] = [];
    let localStepIndex: number = -1;

    exploreWF((step) => {
        if (step.workflow_type !== currentStep.workflow_type) {
            return false;
        }

        localWorkflow.push(step);

        if (step.completed) {
            localStepIndex++;
        }

        return false;
    });

    const ColorlibConnector = withStyles({
        alternativeLabel: {
            top: 22,
        },
        active: {
            "& $line": {
                backgroundImage:
                    "linear-gradient( 95deg,rgb(242,113,33) 0%,rgb(233,64,87) 50%,rgb(138,35,135) 100%)",
            },
        },
        completed: {
            "& $line": {
                backgroundImage:
                    "linear-gradient( 95deg,rgb(242,113,33) 0%,rgb(233,64,87) 50%,rgb(138,35,135) 100%)",
            },
        },
        line: {
            height: 3,
            border: 0,
            backgroundColor: "#eaeaf0",
            borderRadius: 1,
        },
    })(StepConnector);

    const useColorlibStepIconStyles = makeStyles({
        root: {
            backgroundColor: "#ccc",
            zIndex: 1,
            color: "#fff",
            width: 50,
            height: 50,
            display: "flex",
            borderRadius: "50%",
            justifyContent: "center",
            alignItems: "center",
        },
        active: {
            backgroundImage:
                "linear-gradient( 136deg, rgb(242,113,33) 0%, rgb(233,64,87) 50%, rgb(138,35,135) 100%)",
            boxShadow: "0 4px 10px 0 rgba(0,0,0,.25)",
        },
        completed: {
            backgroundImage:
                "linear-gradient( 136deg, rgb(242,113,33) 0%, rgb(233,64,87) 50%, rgb(138,35,135) 100%)",
        },
    });

    function ColorlibStepIcon(props: StepIconProps) {
        const classes = useColorlibStepIconStyles();
        const { active, completed } = props;

        // const icons: { [index: string]: React.ReactElement } = {
        //     1: <BorrowIcon />,
        //     2: <WorkerIcon />,
        //     3: <HouseIcon />,
        // };

        let icons: { [index: string]: React.ReactElement } = {};

        localWorkflow.forEach((step, index) => {
            let iconIndex = index + 1;
            let icon: React.ReactElement = <div />;
            switch (step.actor_role) {
                case "BORROWER":
                    icon = <BorrowIcon />;
                    break;

                case "LIBWORKER":
                    icon = <WorkerIcon />;
                    break;

                case "KEEPER":
                    icon = <HouseIcon />;
                    break;

                default:
                    break;
            }
            icons[iconIndex] = icon;
        });

        return (
            <div
                className={clsx(classes.root, {
                    [classes.active]: active,
                    [classes.completed]: completed,
                })}
            >
                {icons[String(props.icon)]}
            </div>
        );
    }
    const handleStep = async (nextStepIndex: number, backward: boolean) => {
        const step = workflow[nextStepIndex];

        if (!confirm(TEXT["CONFIRM_STEP"] + TEXT["ST_" + step.status])) {
            return;
        }

        const request: WorkflowRequest = {
            master_key: borrow.relations_keys.master,
            act_user: currentUser.username,
            next_step_index: nextStepIndex,
            backward:backward,
        };

        setLoading(true);
        const data = await Client4.doFetch<Result>(
            `/plugins/${manifest.id}/workflow`,
            {
                method: "POST",
                body: JSON.stringify(request),
            }
        );
        setLoading(false);

        if (data.error) {
            setMsgBox({
                open: true,
                text: TEXT["WORKFLOW_ERROR"] + data.error,
                serverity: "error",
            });
            console.error(data);
            return;
        }

        setMsgBox({
            open: true,
            text: TEXT["WORKFLOW_SUCC"],
            serverity: "success",
        });
    };

    const addStepButtons = () => {
        let btns: React.ReactElement[] = [];

        if (
            currentStep.actor_role === "LIBWORKER" &&
            currentStep.status !== STATUS_REQUESTED
        ) {
            btns.push(
                <StyledWFButton
                    variant={"contained"}
                    color={"secondary"}
                    onClick={() => handleStep(currentStep.last_step_index, true)}
                >
                    {TEXT["REJECT"]}
                </StyledWFButton>
            );
        }

        for (let n of currentStep.next_step_index) {
            btns.push(
                <StyledWFButton
                    variant={"contained"}
                    color={"primary"}
                    onClick={() => handleStep(n, false)}
                >
                    {TEXT["ST_" + workflow[n].status]}
                </StyledWFButton>
            );
        }

        return btns;
    };

    const showWfButtons = () => {
        if (
            currentStep.actor_role === borrow.role ||
            borrow.role === "MASTER"
        ) {
            return true;
        }
        return false;
    };

    const borWorkflow = (
        <Grid container direction={"column"}>
            <StyledWFTypeChip
                variant={"outlined"}
                color="primary"
                label={TEXT["WF_" + currentStep.workflow_type]}
            />
            <StyledWFStepper
                alternativeLabel
                activeStep={localStepIndex}
                connector={<ColorlibConnector />}
            >
                {localWorkflow.map((step) => (
                    <Step key={step.status}>
                        <StepLabel StepIconComponent={ColorlibStepIcon}>
                            {TEXT["ST_" + step.status]}
                        </StepLabel>
                    </Step>
                ))}
            </StyledWFStepper>
            {showWfButtons() && <Grid container>{addStepButtons()}</Grid>}
        </Grid>
    );

    const operation = (
        <Grid container direction={"column"} justifyContent={"flex-end"}>
            <Grid item>{dateOfRequest}</Grid>
            <Grid item>{borWorkflow}</Grid>
        </Grid>
    );

    const main = (
        <Grid container spacing={2}>
            <StyledImgWrapper item xs={matchesSm ? 2 : 4}>
                <img
                    src={`${config.SiteURL}/plugins/${manifest.id}/public/info/${borrowReq.book_id}/cover.jpeg`}
                />
            </StyledImgWrapper>
            <Grid item xs={matchesSm ? 4 : 8}>
                {requestInfo}
            </Grid>
            <Grid item xs={matchesSm ? 6 : 12}>
                {operation}
            </Grid>
        </Grid>
    );

    const StyledPaper = styled(Paper)(({ theme }) => ({
        padding: theme.spacing(2),
        margin: "auto",
        maxWidth: "100%",
        position: "relative",
        "& svg": {
            fontSize: "1rem",
        },
    }));

    return (
        <StyledPaper>
            <Grid container direction={"column"}>
                <Grid item>{titleBar}</Grid>
                <Grid item>{main}</Grid>
            </Grid>
            <InProgress open={loading} />
            <MsgBox {...msgBox} close={onCloseMsg} />
        </StyledPaper>
    );
}

BorrowType.propTypes = {
    post: PropTypes.object.isRequired,
    theme: PropTypes.object.isRequired,
};

export default React.memo(BorrowType);
